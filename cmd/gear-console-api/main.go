package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"log"

	"gear/internal/consoleapi"
)

func main() {
	addr := getenv("GEAR_ADDR", ":8080")
	server := &http.Server{
		Addr: addr,
		Handler: consoleapi.NewHandler(consoleapi.Config{
			ConsoleDir:     consoleapi.ExistingConsoleDir(os.Getenv("GEAR_CONSOLE_DIR")),
			EvidenceRoot:   getenv("GEAR_EVIDENCE_ROOT", "evidence"),
			LatencyTrials:  getenvInt("GEAR_LATENCY_TRIALS", 200),
			LatencyWorkers: getenvInt("GEAR_LATENCY_WORKERS", 4),
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("gear-console-api starting", "addr", addr)
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

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
