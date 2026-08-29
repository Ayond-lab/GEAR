package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gear/internal/policy"
)

func main() {
	addr := getenv("GEAR_ADDR", ":8080")
	auditURL := getenv("GEAR_AUDIT_URL", "http://127.0.0.1:8081")

	adjudicator := policy.NewAdjudicator(policy.DefaultCVRuntimePolicy(), policy.NewHTTPAuditClient(auditURL))
	server := &http.Server{
		Addr:              addr,
		Handler:           policy.NewHandler(adjudicator),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("gear-policy starting", "addr", addr, "auditURL", auditURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
