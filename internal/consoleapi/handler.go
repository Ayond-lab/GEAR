package consoleapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gearv1 "gear/api/v1"
	"gear/internal/auditprivacy"
	"gear/internal/chain"
	"gear/internal/cvdemo"
	"gear/internal/evidencepack"
	"gear/internal/latency"
	"gear/internal/mandatederive"
)

const unlawfulPurposeLabel = "Protected citizenship selection purpose"

type Config struct {
	ConsoleDir     string
	EvidenceRoot   string
	LatencyTrials  int
	LatencyWorkers int
	Now            func() time.Time
}

type Server struct {
	config    Config
	mu        sync.RWMutex
	decisions map[string]humanDecision
}

type humanDecision struct {
	Decision  string `json:"decision"`
	DecidedBy string `json:"decidedBy"`
	DecidedAt string `json:"decidedAt"`
	ReasonRef string `json:"reasonRef"`
}

type MandateView struct {
	AbilityRef          string                  `json:"abilityRef"`
	AbilityVersion      string                  `json:"abilityVersion"`
	ManifestDigest      string                  `json:"manifestDigest"`
	RefusedPurposeLabel string                  `json:"refusedPurposeLabel"`
	Refusal             *mandatederive.Refusal  `json:"refusal"`
	RefusalAuditRef     string                  `json:"refusalAuditRef"`
	RefusalAuditEntries []chain.Entry           `json:"refusalAuditEntries"`
	NarrowedMandate     *gearv1.Mandate         `json:"narrowedMandate"`
	Clauses             []mandatederive.Clause  `json:"clauses"`
	ActionGrants        []gearv1.ActionGrant    `json:"actionGrants"`
	ConnectorGrants     []gearv1.ConnectorScope `json:"connectorGrants"`
	Thresholds          map[string]string       `json:"thresholds"`
	Caps                gearv1.Caps             `json:"caps"`
	PolicyFields        []string                `json:"policyFields"`
	HiddenInputs        []string                `json:"hiddenInputs"`
}

type EscalationView struct {
	Summary RunCounts       `json:"summary"`
	Items   []EscalationRow `json:"items"`
}

type RunCounts struct {
	Applications       int `json:"applications"`
	TriggeredActions   int `json:"triggeredActions"`
	Authorised         int `json:"authorised"`
	Escalated          int `json:"escalated"`
	PendingEscalations int `json:"pendingEscalations"`
}

type EscalationRow struct {
	ActionRef     string         `json:"actionRef"`
	ApplicationID string         `json:"applicationId"`
	Confidence    string         `json:"confidence"`
	RuleFired     gearv1.RuleRef `json:"ruleFired"`
	EvidenceRefs  []string       `json:"evidenceRefs"`
	ApproverSet   []string       `json:"approverSet"`
	Status        string         `json:"status"`
	DecidedBy     string         `json:"decidedBy,omitempty"`
	DecidedAt     string         `json:"decidedAt,omitempty"`
	ReasonRef     string         `json:"reasonRef,omitempty"`
}

type DecisionRequest struct {
	ActionRef string `json:"actionRef"`
	Decision  string `json:"decision"`
	DecidedBy string `json:"decidedBy"`
	ReasonRef string `json:"reasonRef"`
}

type AuditView struct {
	Summary                cvdemo.RunSummary      `json:"summary"`
	Verification           chain.Verification     `json:"verification"`
	EffectsWithoutDecision []string               `json:"effectsWithoutDecision"`
	PrivacyFindings        []auditprivacy.Finding `json:"privacyFindings"`
	Entries                []chain.Entry          `json:"entries"`
}

type PrivacyScanView struct {
	Scanned []ScannedArtifact `json:"scanned"`
	OK      bool              `json:"ok"`
}

type ScannedArtifact struct {
	Name     string                 `json:"name"`
	Findings []auditprivacy.Finding `json:"findings"`
}

func NewHandler(config Config) http.Handler {
	server := &Server{config: normaliseConfig(config), decisions: map[string]humanDecision{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.healthz)
	mux.HandleFunc("/api/mandate", server.mandate)
	mux.HandleFunc("/api/escalations", server.escalations)
	mux.HandleFunc("/api/escalations/decision", server.decideEscalation)
	mux.HandleFunc("/api/audit", server.audit)
	mux.HandleFunc("/api/evidence", server.evidence)
	mux.HandleFunc("/api/privacy-scan", server.privacyScan)
	mux.HandleFunc("/api/latency", server.latency)
	mux.Handle("/", server.static())
	return mux
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"component": "gear-console-api", "status": "ok"})
}

func (s *Server) mandate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	view, err := s.mandateView(r.Context())
	if err != nil {
		http.Error(w, "mandate view unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) escalations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	view, err := s.escalationView(r.Context())
	if err != nil {
		http.Error(w, "escalation view unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) decideEscalation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request DecisionRequest
	if err := decodeStrict(r, &request); err != nil {
		http.Error(w, "invalid decision request", http.StatusBadRequest)
		return
	}
	if err := validateDecisionRequest(request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	view, err := s.escalationView(r.Context())
	if err != nil {
		http.Error(w, "escalation view unavailable", http.StatusServiceUnavailable)
		return
	}
	found := false
	for _, item := range view.Items {
		if item.ActionRef == request.ActionRef {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "unknown escalation actionRef", http.StatusNotFound)
		return
	}

	decision := humanDecision{
		Decision:  request.Decision,
		DecidedBy: request.DecidedBy,
		DecidedAt: s.now().Format(time.RFC3339Nano),
		ReasonRef: request.ReasonRef,
	}
	s.mu.Lock()
	s.decisions[request.ActionRef] = decision
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, decision)
}

func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result, err := cvdemo.RunRecordAnnotationPath(r.Context(), s.now)
	if err != nil {
		http.Error(w, "audit view unavailable", http.StatusServiceUnavailable)
		return
	}
	auditData, _ := json.Marshal(result.AuditEntries)
	writeJSON(w, http.StatusOK, AuditView{
		Summary:                result.Summary,
		Verification:           result.ChainVerification,
		EffectsWithoutDecision: result.EffectsWithoutDecision,
		PrivacyFindings:        auditprivacy.Scan(string(auditData)),
		Entries:                result.AuditEntries,
	})
}

func (s *Server) evidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	manifest, err := evidencepack.BuildManifest(s.config.EvidenceRoot, s.now())
	if err != nil {
		http.Error(w, "evidence status unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, manifest)
}

func (s *Server) privacyScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	view, err := s.privacyScanView(r.Context())
	if err != nil {
		http.Error(w, "privacy scan unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) latency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result, err := latency.Run(r.Context(), latency.Config{
		Trials:           s.config.LatencyTrials,
		InferenceWorkers: s.config.LatencyWorkers,
		Now:              s.now,
	})
	if err != nil {
		http.Error(w, "latency run unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) static() http.Handler {
	files := http.FileServer(http.Dir(s.config.ConsoleDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := r.URL.Path
		if path == "/" {
			http.ServeFile(w, r, filepath.Join(s.config.ConsoleDir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})
}

func (s *Server) mandateView(ctx context.Context) (MandateView, error) {
	audit := &cvdemo.MemoryAudit{Now: s.now}
	deriver := mandatederive.NewDeriver(audit)
	deriver.Now = s.now
	ability := cvdemo.CVScreenAbilitySpec()
	refused, err := deriver.Derive(ctx, mandatederive.Request{
		MandateID:           "MND-CONSOLE-REFUSED",
		Version:             1,
		AbilityRef:          cvdemo.AbilityRef,
		Ability:             ability,
		PurposeStatement:    "Check the CVs, select the candidates who are not citizens of the EEA.",
		OperatorResponseRef: "sha256:console-operator-response",
	})
	if err != nil {
		return MandateView{}, err
	}
	accepted, err := deriver.Derive(ctx, mandatederive.Request{
		MandateID:        cvdemo.MandateRef,
		Version:          cvdemo.MandateVersion,
		AbilityRef:       cvdemo.AbilityRef,
		Ability:          ability,
		PurposeStatement: "Identify candidates who will require work authorisation, for planning.",
		LegalBasis:       "Right-to-work verification",
		ExpiresAt:        "2027-02-01T00:00:00Z",
	})
	if err != nil {
		return MandateView{}, err
	}
	view := MandateView{
		AbilityRef:          cvdemo.AbilityRef,
		AbilityVersion:      cvdemo.AbilityVersion,
		ManifestDigest:      ability.ManifestDigest,
		RefusedPurposeLabel: unlawfulPurposeLabel,
		Refusal:             refused.Refusal,
		RefusalAuditRef:     refused.AuditRef,
		RefusalAuditEntries: audit.Snapshot(),
		NarrowedMandate:     accepted.Mandate,
		Clauses:             accepted.Clauses,
		PolicyFields:        []string{"actionClass", "abilityRef", "abilityVersion", "mandateRef", "mandateVersion", "confidence", "dataClasses", "reversibility", "counters", "payloadDigest"},
		HiddenInputs:        []string{"modelOutput", "promptText", "extractedFreeText", "abilityNarrative"},
	}
	if accepted.Mandate != nil {
		view.ActionGrants = accepted.Mandate.Spec.ActionGrants
		view.ConnectorGrants = accepted.Mandate.Spec.ConnectorGrants
		view.Thresholds = accepted.Mandate.Spec.Thresholds
		view.Caps = accepted.Mandate.Spec.Caps
	}
	return view, nil
}

func (s *Server) escalationView(ctx context.Context) (EscalationView, error) {
	run, err := cvdemo.RunRecordAnnotationPath(ctx, s.now)
	if err != nil {
		return EscalationView{}, err
	}
	s.mu.RLock()
	decisions := make(map[string]humanDecision, len(s.decisions))
	for key, value := range s.decisions {
		decisions[key] = value
	}
	s.mu.RUnlock()
	rows := make([]EscalationRow, 0, len(run.Escalations))
	for _, action := range run.Actions {
		if action.Decision.Decision != "escalate" {
			continue
		}
		row := EscalationRow{
			ActionRef:     action.ActionRef,
			ApplicationID: action.ApplicationID,
			Confidence:    action.Confidence,
			RuleFired:     action.Status.RuleFired,
			EvidenceRefs:  action.Extraction.EvidenceRefs,
			ApproverSet:   []string{"hiring-manager-1"},
			Status:        "pending",
		}
		if decision, ok := decisions[action.ActionRef]; ok {
			row.Status = decision.Decision
			row.DecidedBy = decision.DecidedBy
			row.DecidedAt = decision.DecidedAt
			row.ReasonRef = decision.ReasonRef
		}
		rows = append(rows, row)
	}
	return EscalationView{
		Summary: RunCounts{
			Applications:       run.Summary.Applications,
			TriggeredActions:   run.Summary.TriggeredActions,
			Authorised:         run.Summary.Authorised,
			Escalated:          run.Summary.Escalated,
			PendingEscalations: countPending(rows),
		},
		Items: rows,
	}, nil
}

func (s *Server) privacyScanView(ctx context.Context) (PrivacyScanView, error) {
	run, err := cvdemo.RunRecordAnnotationPath(ctx, s.now)
	if err != nil {
		return PrivacyScanView{}, err
	}
	auditData, _ := json.Marshal(run.AuditEntries)
	logs := fmt.Sprintf("{\"component\":\"gear-audit\",\"event\":\"verify\",\"ok\":%t,\"entryCount\":%d}\n", run.ChainVerification.OK, len(run.AuditEntries))
	scanned := []ScannedArtifact{
		{Name: "audit-entries", Findings: auditprivacy.Scan(string(auditData))},
		{Name: "structured-logs", Findings: auditprivacy.Scan(logs)},
	}
	ok := true
	for _, item := range scanned {
		if len(item.Findings) != 0 {
			ok = false
		}
	}
	return PrivacyScanView{Scanned: scanned, OK: ok}, nil
}

func validateDecisionRequest(request DecisionRequest) error {
	if request.ActionRef == "" {
		return errors.New("actionRef is required")
	}
	if request.DecidedBy != "hiring-manager-1" {
		return errors.New("decidedBy must be a mandate approver")
	}
	if request.ReasonRef == "" || !strings.HasPrefix(request.ReasonRef, "fixture://synthetic-cv-lab/reasons/") {
		return errors.New("reasonRef must point to the synthetic fixture store")
	}
	switch request.Decision {
	case "approve", "decline", "info":
		return nil
	default:
		return errors.New("decision must be approve, decline, or info")
	}
}

func countPending(rows []EscalationRow) int {
	count := 0
	for _, row := range rows {
		if row.Status == "pending" {
			count++
		}
	}
	return count
}

func decodeStrict(r *http.Request, into any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("request body must contain one json document")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func normaliseConfig(config Config) Config {
	if config.ConsoleDir == "" {
		config.ConsoleDir = "console"
	}
	if config.EvidenceRoot == "" {
		config.EvidenceRoot = "evidence"
	}
	if config.LatencyTrials <= 0 {
		config.LatencyTrials = 200
	}
	if config.LatencyWorkers <= 0 {
		config.LatencyWorkers = 4
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	return config
}

func (s *Server) now() time.Time {
	return s.config.Now().UTC()
}

func ExistingConsoleDir(path string) string {
	if path != "" {
		return path
	}
	for _, candidate := range []string{"console", "/console"} {
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			return candidate
		}
	}
	return "console"
}
