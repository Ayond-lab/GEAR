package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gearv1 "gear/api/v1"
	"gear/internal/mandatederive"
	"gear/internal/mandatesign"
	"gear/internal/policy"

	"sigs.k8s.io/yaml"
)

func main() {
	mode := flag.String("mode", getenv("GEAR_MODE", "serve"), "serve or sign-file")
	input := flag.String("input", "", "input Mandate YAML for sign-file mode")
	output := flag.String("output", "", "output Mandate YAML for sign-file mode; stdout when empty")
	addr := flag.String("listen", getenv("GEAR_ADDR", ":8080"), "listen address")
	auditURL := flag.String("audit-url", getenv("GEAR_AUDIT_URL", "http://gear-audit.gear-system.svc.cluster.local:8080"), "gear-audit base URL")
	flag.Parse()

	switch *mode {
	case "serve":
		if err := serve(*addr, *auditURL); err != nil {
			log.Fatal(err)
		}
	case "sign-file":
		if err := signFile(*input, *output); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown mode %q", *mode)
	}
}

func serve(addr, auditURL string) error {
	deriver := mandatederive.NewDeriver(policy.NewHTTPAuditClient(auditURL))
	server := &http.Server{
		Addr:              addr,
		Handler:           mandatederive.NewHandler(deriver),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("gear-mandate starting", "addr", addr, "auditURL", auditURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func signFile(inputPath, outputPath string) error {
	if inputPath == "" {
		return fmt.Errorf("--input is required in sign-file mode")
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	var mandate gearv1.Mandate
	if err := yaml.Unmarshal(data, &mandate); err != nil {
		return err
	}
	signature, err := mandatesign.Sign(mandate.Spec, mandatesign.DevelopmentPrivateKey())
	if err != nil {
		return err
	}
	mandate.Spec.Signature = signature
	out, err := yaml.Marshal(mandate)
	if err != nil {
		return err
	}
	if outputPath == "" {
		_, err = os.Stdout.Write(out)
		return err
	}
	return os.WriteFile(outputPath, out, 0o644)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
