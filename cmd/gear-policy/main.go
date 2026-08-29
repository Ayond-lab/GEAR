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

	"gear/internal/exectoken"
	"gear/internal/mtls"
	"gear/internal/policy"
)

func main() {
	addr := getenv("GEAR_ADDR", ":8080")
	healthAddr := getenv("GEAR_HEALTH_ADDR", ":8081")
	auditURL := getenv("GEAR_AUDIT_URL", "http://127.0.0.1:8081")
	tlsCert := os.Getenv("GEAR_TLS_CERT")
	tlsKey := os.Getenv("GEAR_TLS_KEY")
	clientCA := os.Getenv("GEAR_CLIENT_CA")
	tokenPrivateKey := os.Getenv("GEAR_EXEC_TOKEN_PRIVATE_KEY")

	adjudicator := policy.NewAdjudicator(policy.DefaultCVRuntimePolicy(), policy.NewHTTPAuditClient(auditURL))
	if tokenPrivateKey != "" {
		key, err := exectoken.LoadPrivateKey(tokenPrivateKey)
		if err != nil {
			log.Fatal(err)
		}
		adjudicator.SetExecutionTokenPrivateKey(key)
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           policy.NewHandler(adjudicator),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serve := server.ListenAndServe
	var healthServer *http.Server
	if mtls.AllFilesExist(tlsCert, tlsKey, clientCA) {
		tlsConfig, err := mtls.ServerConfig(clientCA)
		if err != nil {
			log.Fatal(err)
		}
		server.TLSConfig = tlsConfig
		serve = func() error { return server.ListenAndServeTLS(tlsCert, tlsKey) }
		healthServer = &http.Server{
			Addr:              healthAddr,
			Handler:           policy.NewHealthHandler(),
			ReadHeaderTimeout: 5 * time.Second,
		}
	} else if anySet(tlsCert, tlsKey, clientCA) {
		log.Fatal("gear-policy mTLS configuration is incomplete")
	}

	go func() {
		slog.Info("gear-policy starting", "addr", addr, "auditURL", auditURL, "mtls", healthServer != nil)
		if err := serve(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	if healthServer != nil {
		go func() {
			slog.Info("gear-policy health starting", "addr", healthAddr)
			if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatal(err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if healthServer != nil {
		_ = healthServer.Shutdown(shutdownCtx)
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}

func anySet(values ...string) bool {
	for _, value := range values {
		if value != "" {
			return true
		}
	}
	return false
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
