package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gear/internal/auditservice"
	"gear/internal/chain"
)

func main() {
	addr := getenv("GEAR_ADDR", ":8080")
	dbPath := getenv("GEAR_AUDIT_DB", filepath.Join("data", "gear-audit.db"))

	store, err := chain.OpenBoltStore(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	server := &http.Server{
		Addr:              addr,
		Handler:           auditservice.NewHandler(store),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("gear-audit starting", "addr", addr, "db", dbPath)
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
