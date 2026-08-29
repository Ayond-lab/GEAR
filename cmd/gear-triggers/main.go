package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gear/internal/chain"
	"gear/internal/cvdemo"
	"gear/internal/policy"
)

type triggerSyncResponse struct {
	Plan              cvdemo.TriggerPlan `json:"plan"`
	AuditRecorded     bool               `json:"auditRecorded"`
	NonMatchAuditRefs []string           `json:"nonMatchAuditRefs"`
}

func main() {
	addr := getenv("GEAR_ADDR", ":8080")
	auditURL := os.Getenv("GEAR_AUDIT_URL")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"component": "gear-triggers", "status": "ok"})
	})
	mux.HandleFunc("/v1/plan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, cvdemo.BuildRecordAnnotationPlan(cvdemo.GenerateApplications()))
	})
	mux.HandleFunc("/v1/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		plan := cvdemo.BuildRecordAnnotationPlan(cvdemo.GenerateApplications())
		response := triggerSyncResponse{Plan: plan}
		if auditURL != "" {
			client := policy.NewHTTPAuditClient(auditURL)
			for _, nonMatch := range plan.NonMatches {
				stored, err := client.Append(r.Context(), cvdemo.NonMatchAuditEntry(nonMatch, time.Now().UTC()))
				if err != nil {
					http.Error(w, fmt.Sprintf("non-match audit append failed: %v", err), http.StatusServiceUnavailable)
					return
				}
				response.NonMatchAuditRefs = append(response.NonMatchAuditRefs, chain.Ref(stored.Seq))
			}
			response.AuditRecorded = true
		}
		writeJSON(w, http.StatusOK, response)
	})

	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		slog.Info("gear-triggers starting", "addr", addr, "auditURL", auditURL)
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
