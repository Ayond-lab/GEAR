package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestMutateAbilityPodInjectsPEPAndEgressControls(t *testing.T) {
	pod := abilityPod()

	changed := MutateAbilityPod(pod)

	if !changed {
		t.Fatal("expected ability pod to be mutated")
	}
	if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		t.Fatal("expected pod service account token automount to be disabled")
	}

	ability := findContainer(pod.Spec.Containers, "ability")
	if ability == nil {
		t.Fatal("expected ability container to remain present")
	}
	if ability.SecurityContext == nil || ability.SecurityContext.RunAsUser == nil || *ability.SecurityContext.RunAsUser != AbilityContainerUID {
		t.Fatalf("expected ability container UID %d, got %#v", AbilityContainerUID, ability.SecurityContext)
	}
	assertNoCredentialMounts(t, *ability)

	pep := findContainer(pod.Spec.Containers, PEPContainerName)
	if pep == nil {
		t.Fatal("expected PEP sidecar to be injected")
	}
	if pep.SecurityContext == nil || pep.SecurityContext.RunAsUser == nil || *pep.SecurityContext.RunAsUser != PEPContainerUID {
		t.Fatalf("expected PEP UID %d, got %#v", PEPContainerUID, pep.SecurityContext)
	}
	assertVolumeMount(t, *pep, PEPMTLSVolumeName, PEPMTLSMountPath)
	assertVolumeMount(t, *pep, ConnectorCredentialsVolumeName, ConnectorCredentialsMountPath)

	init := findContainer(pod.Spec.InitContainers, EgressInitContainerName)
	if init == nil {
		t.Fatal("expected UID egress init container to be injected")
	}
	if init.SecurityContext == nil || init.SecurityContext.Capabilities == nil || !hasCapability(init.SecurityContext.Capabilities.Add, "NET_ADMIN") {
		t.Fatalf("expected init container to have NET_ADMIN capability, got %#v", init.SecurityContext)
	}
	egressRules := strings.Join(init.Args, "\n")
	for _, required := range []string{"-o lo", "--uid-owner 1337", "REJECT --reject-with icmp-admin-prohibited"} {
		if !strings.Contains(egressRules, required) {
			t.Fatalf("expected egress rule fragment %q in %q", required, egressRules)
		}
	}

	assertVolume(t, pod.Spec.Volumes, PEPMTLSVolumeName, PEPMTLSSecretName)
	assertVolume(t, pod.Spec.Volumes, ConnectorCredentialsVolumeName, ConnectorCredentialsSecretName)
}

func TestMutateAbilityPodIgnoresUnlabelledPods(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "plain"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "example/app:dev"}},
		},
	}

	changed := MutateAbilityPod(pod)

	if changed {
		t.Fatal("expected unlabelled pod to be ignored")
	}
	if findContainer(pod.Spec.Containers, PEPContainerName) != nil {
		t.Fatal("did not expect PEP sidecar on unlabelled pod")
	}
}

func TestMutateAbilityPodIsIdempotent(t *testing.T) {
	pod := abilityPod()

	MutateAbilityPod(pod)
	MutateAbilityPod(pod)

	if got := countContainers(pod.Spec.Containers, PEPContainerName); got != 1 {
		t.Fatalf("expected one PEP sidecar, got %d", got)
	}
	if got := countContainers(pod.Spec.InitContainers, EgressInitContainerName); got != 1 {
		t.Fatalf("expected one egress init container, got %d", got)
	}
	if got := countVolumes(pod.Spec.Volumes, PEPMTLSVolumeName); got != 1 {
		t.Fatalf("expected one PEP mTLS volume, got %d", got)
	}
}

func TestPodMutatorRejectsUnexpectedObject(t *testing.T) {
	err := NewPodMutator().Default(context.Background(), narrowedMandate())
	if !errors.Is(err, ErrUnexpectedObject) {
		t.Fatalf("expected unexpected object error, got %v", err)
	}
}

func TestPodAdmissionHandlerPatchesAbilityPod(t *testing.T) {
	scheme := podScheme(t)
	handler := admission.WithCustomDefaulter(scheme, &corev1.Pod{}, NewPodMutator())

	response := handler.Handle(context.Background(), podAdmissionRequest(t, abilityPod()))

	if !response.Allowed {
		t.Fatalf("expected pod mutation admission to allow request, got %#v", response.Result)
	}
	if len(response.Patches) == 0 {
		t.Fatal("expected ability pod admission to return JSON patches")
	}
}

func TestPodAdmissionHandlerLeavesPlainPodUnpatched(t *testing.T) {
	scheme := podScheme(t)
	handler := admission.WithCustomDefaulter(scheme, &corev1.Pod{}, NewPodMutator())

	response := handler.Handle(context.Background(), podAdmissionRequest(t, &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: "plain", Namespace: "gear-lab"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "example/app:dev"}},
		},
	}))

	if !response.Allowed {
		t.Fatalf("expected plain pod admission to allow request, got %#v", response.Result)
	}
	if len(response.Patches) != 0 {
		t.Fatalf("expected no patches for plain pod, got %#v", response.Patches)
	}
}

func abilityPod() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cv-screen-run",
			Namespace: "gear-lab",
			Labels: map[string]string{
				AbilityLabel: "cv-screen",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "ability", Image: "example/cv-screen:dev"},
			},
		},
	}
}

func podScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func podAdmissionRequest(t *testing.T, pod *corev1.Pod) admission.Request {
	t.Helper()
	data, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "pod-request",
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: data,
			},
		},
	}
}

func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for index := range containers {
		if containers[index].Name == name {
			return &containers[index]
		}
	}
	return nil
}

func countContainers(containers []corev1.Container, name string) int {
	count := 0
	for _, container := range containers {
		if container.Name == name {
			count++
		}
	}
	return count
}

func countVolumes(volumes []corev1.Volume, name string) int {
	count := 0
	for _, volume := range volumes {
		if volume.Name == name {
			count++
		}
	}
	return count
}

func assertNoCredentialMounts(t *testing.T, container corev1.Container) {
	t.Helper()
	for _, mount := range container.VolumeMounts {
		if mount.Name == PEPMTLSVolumeName || mount.Name == ConnectorCredentialsVolumeName {
			t.Fatalf("credential volume %q must not be mounted into ability container", mount.Name)
		}
	}
}

func assertVolumeMount(t *testing.T, container corev1.Container, name string, path string) {
	t.Helper()
	for _, mount := range container.VolumeMounts {
		if mount.Name == name && mount.MountPath == path && mount.ReadOnly {
			return
		}
	}
	t.Fatalf("expected read-only mount %s at %s in %#v", name, path, container.VolumeMounts)
}

func assertVolume(t *testing.T, volumes []corev1.Volume, name string, secretName string) {
	t.Helper()
	for _, volume := range volumes {
		if volume.Name == name && volume.Secret != nil && volume.Secret.SecretName == secretName {
			return
		}
	}
	t.Fatalf("expected secret volume %s from secret %s in %#v", name, secretName, volumes)
}

func hasCapability(capabilities []corev1.Capability, name string) bool {
	for _, capability := range capabilities {
		if string(capability) == name {
			return true
		}
	}
	return false
}
