//go:build envtest

package webhooks

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gearv1 "gear/api/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func TestEnvtestMandateAdmissionRejectsWidenedMandate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := gearv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "deploy", "base", "crds")},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "deploy", "base", "webhook-validatingconfiguration.yaml")},
		},
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("start envtest control plane: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		if err := testEnv.Stop(); err != nil {
			t.Errorf("stop envtest control plane: %v", err)
		}
	})

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
	})
	if err != nil {
		t.Fatalf("create webhook manager: %v", err)
	}
	if err := RegisterMandateWebhookWithManager(mgr.GetWebhookServer(), mgr.GetScheme(), mgr.GetAPIReader()); err != nil {
		t.Fatalf("register mandate webhook: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- mgr.Start(ctx)
	}()
	waitForWebhookServer(t, mgr.GetWebhookServer(), errCh)

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("create envtest client: %v", err)
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gear-lab",
		},
	}
	if err := k8sClient.Create(ctx, namespace); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create namespace: %v", err)
	}
	if err := k8sClient.Create(ctx, cvScreenAbility()); err != nil {
		t.Fatalf("create referenced ability: %v", err)
	}

	if err := k8sClient.Create(ctx, narrowedMandate()); err != nil {
		t.Fatalf("expected narrowed mandate to pass admission, got %v", err)
	}

	widened := narrowedMandate()
	widened.Name = "mnd-widened"
	widened.Spec.MandateID = "MND-WIDENED"
	widened.Spec.ActionGrants = append(widened.Spec.ActionGrants, gearv1.ActionGrant{
		Class:       "DELETE_RECORD",
		Disposition: "permit",
	})

	err = k8sClient.Create(ctx, widened)
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected widened mandate to be rejected by API admission, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "action class is outside manifest") {
		t.Fatalf("expected subsumption violation in API admission error, got %v", err)
	}

	illegal := narrowedMandate()
	illegal.Name = "mnd-candidate-rank-permit"
	illegal.Spec.MandateID = "MND-CANDIDATE-RANK-PERMIT"
	illegal.Spec.PurposeStatement = "Check the CVs, select the candidates who are not citizens of the EEA."
	illegal.Spec.ActionGrants = []gearv1.ActionGrant{
		{Class: "RECORD_ANNOTATE", Disposition: "permit"},
		{Class: "CANDIDATE_RANK", Disposition: "permit"},
	}
	illegal.Spec.Signature = mustSignMandateSpec(illegal.Spec)

	err = k8sClient.Create(ctx, illegal)
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected CANDIDATE_RANK permit mandate to be rejected by API admission, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "CANDIDATE_RANK was refused by legality gate") {
		t.Fatalf("expected legality-gate rejection in API admission error, got %v", err)
	}
}

func waitForWebhookServer(t *testing.T, server webhook.Server, errCh <-chan error) {
	t.Helper()

	check := server.StartedChecker()
	deadline := time.Now().Add(10 * time.Second)
	for {
		select {
		case err := <-errCh:
			t.Fatalf("webhook manager stopped before serving: %v", err)
		default:
		}

		if err := check(nil); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("webhook server did not become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
