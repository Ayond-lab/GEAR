package main

import (
	"flag"
	"fmt"
	"os"

	gearv1 "gear/api/v1"
	gearwebhooks "gear/internal/webhooks"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func main() {
	var metricsAddr string
	var probeAddr string
	var certDir string
	var webhookPort int

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. Use 0 to disable metrics.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the health probe endpoint binds to.")
	flag.StringVar(&certDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs", "Directory containing tls.crt and tls.key for the webhook server.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "The secure port the webhook server binds to.")
	flag.Parse()

	scheme := runtime.NewScheme()
	must("add client-go scheme", clientgoscheme.AddToScheme(scheme))
	must("add gear scheme", gearv1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: certDir,
		}),
	})
	must("create manager", err)

	must("register mandate validating webhook", gearwebhooks.RegisterMandateWebhookWithManager(
		mgr.GetWebhookServer(),
		mgr.GetScheme(),
		mgr.GetClient(),
	))
	must("register pod mutating webhook", gearwebhooks.RegisterPodMutationWebhookWithManager(
		mgr.GetWebhookServer(),
		mgr.GetScheme(),
	))
	must("add health check", mgr.AddHealthzCheck("healthz", healthz.Ping))
	must("add readiness check", mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()))
	must("start manager", mgr.Start(ctrl.SetupSignalHandler()))
}

func must(action string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "gear-webhooks: %s: %v\n", action, err)
	os.Exit(1)
}
