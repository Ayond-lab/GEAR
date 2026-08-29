package fixturestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gear/internal/cvdemo"
)

var ErrNonSyntheticNamespace = errors.New("fixture store only accepts synthetic namespaces")

type Store struct {
	namespace   string
	mu          sync.RWMutex
	apps        map[string]applicationRecord
	extractions map[string]ExtractionRecord
	reasons     map[string]ReasonRecord
}

type applicationRecord struct {
	Application cvdemo.Application
	Erased      bool
}

type ApplicationView struct {
	ApplicationID   string `json:"applicationId"`
	SourceRef       string `json:"sourceRef"`
	SubjectRef      string `json:"subjectRef"`
	PayloadDigest   string `json:"payloadDigest"`
	Status          string `json:"workAuthorisationStatus"`
	ApplicationText string `json:"applicationText,omitempty"`
	InjectionCase   bool   `json:"injectionCase"`
	Erased          bool   `json:"erased"`
}

type ExtractionRecord struct {
	Ref           string            `json:"ref"`
	SourceRef     string            `json:"sourceRef"`
	PayloadDigest string            `json:"payloadDigest"`
	Fields        map[string]string `json:"fields"`
	Confidence    string            `json:"confidence"`
	CreatedAt     string            `json:"createdAt"`
}

type StoreExtractionRequest struct {
	SourceRef     string            `json:"sourceRef"`
	PayloadDigest string            `json:"payloadDigest"`
	Fields        map[string]string `json:"fields"`
	Confidence    string            `json:"confidence"`
}

type ReasonRecord struct {
	Ref       string `json:"ref"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"createdAt"`
}

type StoreReasonRequest struct {
	Reason string `json:"reason"`
}

type ListApplicationsResponse struct {
	Namespace    string            `json:"namespace"`
	Applications []ApplicationView `json:"applications"`
}

func New(namespace string, applications []cvdemo.Application) (*Store, error) {
	if err := ValidateNamespace(namespace); err != nil {
		return nil, err
	}
	store := &Store{
		namespace:   namespace,
		apps:        map[string]applicationRecord{},
		extractions: map[string]ExtractionRecord{},
		reasons:     map[string]ReasonRecord{},
	}
	for _, application := range applications {
		store.apps[application.ApplicationID] = applicationRecord{Application: application}
	}
	return store, nil
}

func ValidateNamespace(namespace string) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || (!strings.HasPrefix(namespace, "synthetic-") && namespace != cvdemo.SyntheticNamespace) {
		return ErrNonSyntheticNamespace
	}
	return nil
}

func (s *Store) ListApplications() []ApplicationView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.apps))
	for id := range s.apps {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	views := make([]ApplicationView, 0, len(ids))
	for _, id := range ids {
		views = append(views, viewOf(s.apps[id], false))
	}
	return views
}

func (s *Store) GetApplication(id string) (ApplicationView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.apps[id]
	if !ok {
		return ApplicationView{}, false
	}
	return viewOf(record, true), true
}

func (s *Store) EraseApplication(id string) (ApplicationView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.apps[id]
	if !ok {
		return ApplicationView{}, false
	}
	record.Application.ApplicationText = ""
	record.Application.Salt = ""
	record.Erased = true
	s.apps[id] = record
	return viewOf(record, true), true
}

func (s *Store) StoreExtraction(ctx context.Context, request StoreExtractionRequest) (ExtractionRecord, error) {
	if strings.TrimSpace(request.SourceRef) == "" || strings.TrimSpace(request.PayloadDigest) == "" || len(request.Fields) == 0 || strings.TrimSpace(request.Confidence) == "" {
		return ExtractionRecord{}, errors.New("sourceRef, payloadDigest, fields, and confidence are required")
	}
	ref := extractionRef(request)
	record := ExtractionRecord{
		Ref:           ref,
		SourceRef:     request.SourceRef,
		PayloadDigest: request.PayloadDigest,
		Fields:        cloneFields(request.Fields),
		Confidence:    request.Confidence,
		CreatedAt:     now(ctx).Format(time.RFC3339Nano),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extractions[ref] = record
	return record, nil
}

func (s *Store) StoreReason(ctx context.Context, reason string) (ReasonRecord, error) {
	if strings.TrimSpace(reason) == "" {
		return ReasonRecord{}, errors.New("reason is required")
	}
	ref := fmt.Sprintf("fixture://%s/reasons/%s", s.namespace, digestHex(reason))
	record := ReasonRecord{Ref: ref, Reason: reason, CreatedAt: now(ctx).Format(time.RFC3339Nano)}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reasons[ref] = record
	return record, nil
}

func NewHandler(store *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"component": "gear-fixture-store", "status": "ok", "namespace": store.namespace})
	})
	mux.HandleFunc("/v1/applications", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, ListApplicationsResponse{Namespace: store.namespace, Applications: store.ListApplications()})
	})
	mux.HandleFunc("/v1/applications/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/applications/")
		if id == "" || strings.Contains(id, "/") {
			http.Error(w, "invalid application id", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			application, ok := store.GetApplication(id)
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, http.StatusOK, application)
		case http.MethodDelete:
			application, ok := store.EraseApplication(id)
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, http.StatusOK, application)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/extractions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var request StoreExtractionRequest
		if err := decode(r, &request); err != nil {
			http.Error(w, "invalid extraction request", http.StatusBadRequest)
			return
		}
		record, err := store.StoreExtraction(r.Context(), request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, record)
	})
	mux.HandleFunc("/v1/reasons", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var request StoreReasonRequest
		if err := decode(r, &request); err != nil {
			http.Error(w, "invalid reason request", http.StatusBadRequest)
			return
		}
		record, err := store.StoreReason(r.Context(), request.Reason)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, record)
	})
	return mux
}

func viewOf(record applicationRecord, includeText bool) ApplicationView {
	application := record.Application
	view := ApplicationView{
		ApplicationID: application.ApplicationID,
		SourceRef:     cvdemo.SourceRef(application.ApplicationID),
		SubjectRef:    application.SubjectRef,
		PayloadDigest: application.PayloadDigest,
		Status:        application.Status,
		InjectionCase: application.InjectionCase,
		Erased:        record.Erased,
	}
	if includeText && !record.Erased {
		view.ApplicationText = application.ApplicationText
	}
	return view
}

func extractionRef(request StoreExtractionRequest) string {
	data, _ := json.Marshal(struct {
		SourceRef     string            `json:"sourceRef"`
		PayloadDigest string            `json:"payloadDigest"`
		Fields        map[string]string `json:"fields"`
		Confidence    string            `json:"confidence"`
	}{request.SourceRef, request.PayloadDigest, request.Fields, request.Confidence})
	return fmt.Sprintf("fixture://%s/extractions/%s", cvdemo.SyntheticNamespace, digestHex(string(data)))
}

func cloneFields(fields map[string]string) map[string]string {
	clone := make(map[string]string, len(fields))
	for key, value := range fields {
		clone[key] = value
	}
	return clone
}

func decode(r *http.Request, into any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(into)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func digestHex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

type nowKey struct{}

func WithNow(ctx context.Context, now func() time.Time) context.Context {
	return context.WithValue(ctx, nowKey{}, now)
}

func now(ctx context.Context) time.Time {
	if value := ctx.Value(nowKey{}); value != nil {
		if fn, ok := value.(func() time.Time); ok && fn != nil {
			return fn().UTC()
		}
	}
	return time.Now().UTC()
}
