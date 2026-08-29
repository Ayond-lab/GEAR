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

	"gear/internal/cvdemo"
	"gear/internal/fixturestore"
)

func main() {
	addr := getenv("GEAR_ADDR", ":8080")
	namespace := getenv("GEAR_FIXTURE_NAMESPACE", cvdemo.SyntheticNamespace)
	store, err := fixturestore.New(namespace, cvdemo.GenerateApplications())
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           fixturestore.NewHandler(store),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("gear-fixture-store starting", "addr", addr, "namespace", namespace)
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
