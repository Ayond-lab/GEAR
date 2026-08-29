package webhooks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	AbilityLabel = "gear.eu/ability"

	AbilityContainerUID int64 = 1001
	PEPContainerUID     int64 = 1337
	RootUID             int64 = 0

	PEPContainerName        = "gear-pep"
	EgressInitContainerName = "gear-uid-egress"

	PEPImage        = "ghcr.io/ayond-lab/gear-pep:dev"
	EgressInitImage = "ghcr.io/ayond-lab/gear-netinit:dev"

	PEPMTLSVolumeName              = "gear-pep-mtls"
	ConnectorCredentialsVolumeName = "gear-connector-credentials"

	PEPMTLSSecretName              = "gear-pep-mtls"
	ConnectorCredentialsSecretName = "gear-connector-credentials"

	PEPMTLSMountPath              = "/var/run/gear/mtls"
	ConnectorCredentialsMountPath = "/var/run/gear/connectors"
)

type PodMutator struct{}

func NewPodMutator() PodMutator {
	return PodMutator{}
}

func (m PodMutator) Default(_ context.Context, obj runtime.Object) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("%w: %T", ErrUnexpectedObject, obj)
	}
	MutateAbilityPod(pod)
	return nil
}

func MutateAbilityPod(pod *corev1.Pod) bool {
	if pod == nil || !IsAbilityPod(pod) {
		return false
	}

	disabled := false
	pod.Spec.AutomountServiceAccountToken = &disabled

	for index := range pod.Spec.Containers {
		if pod.Spec.Containers[index].Name == PEPContainerName {
			continue
		}
		setAbilityContainerSecurity(&pod.Spec.Containers[index])
	}

	upsertVolume(&pod.Spec.Volumes, secretVolume(PEPMTLSVolumeName, PEPMTLSSecretName))
	upsertVolume(&pod.Spec.Volumes, secretVolume(ConnectorCredentialsVolumeName, ConnectorCredentialsSecretName))
	upsertInitContainer(&pod.Spec.InitContainers, egressInitContainer())
	upsertContainer(&pod.Spec.Containers, pepContainer())

	return true
}

func IsAbilityPod(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return pod.Labels[AbilityLabel] != ""
}

func setAbilityContainerSecurity(container *corev1.Container) {
	if container.SecurityContext == nil {
		container.SecurityContext = &corev1.SecurityContext{}
	}
	runAsNonRoot := true
	allowPrivilegeEscalation := false
	container.SecurityContext.RunAsUser = int64ptr(AbilityContainerUID)
	container.SecurityContext.RunAsNonRoot = &runAsNonRoot
	container.SecurityContext.AllowPrivilegeEscalation = &allowPrivilegeEscalation
}

func pepContainer() corev1.Container {
	runAsNonRoot := true
	allowPrivilegeEscalation := false
	return corev1.Container{
		Name:            PEPContainerName,
		Image:           PEPImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"--listen=127.0.0.1:9191",
		},
		Ports: []corev1.ContainerPort{
			{Name: "pep-loopback", ContainerPort: 9191},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "GEAR_PEP_LISTEN",
				Value: "127.0.0.1:9191",
			},
			{
				Name:  "GEAR_POLICY_URL",
				Value: "https://gear-policy.gear-system.svc.cluster.local:443",
			},
			{
				Name:  "GEAR_AUDIT_URL",
				Value: "http://gear-audit.gear-system.svc.cluster.local:8080",
			},
			{
				Name:  "GEAR_INFERENCE_URL",
				Value: "http://gear-inference.gear-system.svc.cluster.local:8080",
			},
			{
				Name:  "GEAR_POLICY_CLIENT_CERT",
				Value: PEPMTLSMountPath + "/tls.crt",
			},
			{
				Name:  "GEAR_POLICY_CLIENT_KEY",
				Value: PEPMTLSMountPath + "/tls.key",
			},
			{
				Name:  "GEAR_POLICY_CA",
				Value: PEPMTLSMountPath + "/ca.crt",
			},
			{
				Name:  "GEAR_ALLOWED_SCOPES",
				Value: "candidate-record:write",
			},
			{
				Name: "GEAR_ABILITY_REF",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.labels['gear.eu/ability']",
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: PEPMTLSVolumeName, MountPath: PEPMTLSMountPath, ReadOnly: true},
			{Name: ConnectorCredentialsVolumeName, MountPath: ConnectorCredentialsMountPath, ReadOnly: true},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                int64ptr(PEPContainerUID),
			RunAsNonRoot:             &runAsNonRoot,
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		},
	}
}

func egressInitContainer() corev1.Container {
	allowPrivilegeEscalation := false
	runAsNonRoot := false
	return corev1.Container{
		Name:            EgressInitContainerName,
		Image:           EgressInitImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sh", "-c"},
		Args: []string{
			"iptables -A OUTPUT -o lo -j ACCEPT\n" +
				"iptables -A OUTPUT -m owner --uid-owner 1337 -j ACCEPT\n" +
				"iptables -A OUTPUT -j REJECT --reject-with icmp-admin-prohibited",
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                int64ptr(RootUID),
			RunAsNonRoot:             &runAsNonRoot,
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
		},
	}
}

func secretVolume(name, secretName string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
}

func upsertContainer(containers *[]corev1.Container, desired corev1.Container) {
	for index := range *containers {
		if (*containers)[index].Name == desired.Name {
			(*containers)[index] = desired
			return
		}
	}
	*containers = append(*containers, desired)
}

func upsertInitContainer(containers *[]corev1.Container, desired corev1.Container) {
	for index := range *containers {
		if (*containers)[index].Name == desired.Name {
			(*containers)[index] = desired
			return
		}
	}
	*containers = append(*containers, desired)
}

func upsertVolume(volumes *[]corev1.Volume, desired corev1.Volume) {
	for index := range *volumes {
		if (*volumes)[index].Name == desired.Name {
			(*volumes)[index] = desired
			return
		}
	}
	*volumes = append(*volumes, desired)
}

func int64ptr(value int64) *int64 {
	return &value
}

var _ admission.CustomDefaulter = PodMutator{}
