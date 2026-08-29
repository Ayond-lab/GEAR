package auditservice

import (
	"context"
	"encoding/json"
	"net/http"

	"gear/internal/chain"
)

type Store interface {
	Append(ctx context.Context, entry chain.Entry) (chain.Entry, error)
	Entries(ctx context.Context) ([]chain.Entry, error)
	Verify(ctx context.Context) (chain.Verification, error)
	EffectsWithoutDecisions(ctx context.Context) ([]string, error)
}

type AppendResponse struct {
	AuditRef string      `json:"auditRef"`
	Entry    chain.Entry `json:"entry"`
}

type EntriesResponse struct {
	Entries []chain.Entry `json:"entries"`
}

type VerifyResponse struct {
	OK         bool          `json:"ok"`
	EntryCount int           `json:"entryCount"`
	Affected   []chain.Range `json:"affected"`
}

type ReconciliationResponse struct {
	EffectsWithoutDecisions []string `json:"effectsWithoutDecisions"`
}

func NewHandler(store Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"component": "gear-audit", "status": "ok"})
	})
	mux.HandleFunc("/v1/entries", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			appendEntry(w, r, store)
		case http.MethodGet:
			listEntries(w, r, store)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		verifyEntries(w, r, store)
	})
	mux.HandleFunc("/v1/reconcile/effects-without-decisions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reconcileEffects(w, r, store)
	})
	return mux
}

func appendEntry(w http.ResponseWriter, r *http.Request, store Store) {
	defer r.Body.Close()
	var entry chain.Entry
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&entry); err != nil {
		http.Error(w, "invalid audit entry", http.StatusBadRequest)
		return
	}

	stored, err := store.Append(r.Context(), entry)
	if err != nil {
		http.Error(w, "audit append failed", http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusCreated, AppendResponse{
		AuditRef: chain.Ref(stored.Seq),
		Entry:    stored,
	})
}

func listEntries(w http.ResponseWriter, r *http.Request, store Store) {
	entries, err := store.Entries(r.Context())
	if err != nil {
		http.Error(w, "audit read failed", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, EntriesResponse{Entries: entries})
}

func verifyEntries(w http.ResponseWriter, r *http.Request, store Store) {
	entries, err := store.Entries(r.Context())
	if err != nil {
		http.Error(w, "audit read failed", http.StatusServiceUnavailable)
		return
	}
	result := chain.Verify(entries)
	writeJSON(w, http.StatusOK, VerifyResponse{
		OK:         result.OK,
		EntryCount: len(entries),
		Affected:   result.Affected,
	})
}

func reconcileEffects(w http.ResponseWriter, r *http.Request, store Store) {
	missing, err := store.EffectsWithoutDecisions(r.Context())
	if err != nil {
		http.Error(w, "audit reconciliation failed", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, ReconciliationResponse{EffectsWithoutDecisions: missing})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
