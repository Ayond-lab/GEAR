package pepcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const LoopbackListenAddress = "127.0.0.1:9191"

var (
	ErrActiveActionUnavailable = errors.New("active governed action unavailable")
	ErrRequestRejected         = errors.New("pep request rejected")
	ErrExtractorUnavailable    = errors.New("extractor unavailable")
)

type ActiveAction struct {
	ActionRef      string         `json:"actionRef"`
	ActionClass    string         `json:"actionClass"`
	AbilityRef     string         `json:"abilityRef"`
	AbilityVersion string         `json:"abilityVersion"`
	MandateRef     string         `json:"mandateRef"`
	MandateVersion int            `json:"mandateVersion"`
	SubjectRef     string         `json:"subjectRef"`
	DataClasses    []string       `json:"dataClasses"`
	Confidence     string         `json:"confidence"`
	Reversibility  string         `json:"reversibility"`
	Counters       map[string]int `json:"counters"`
	PayloadDigest  string         `json:"payloadDigest"`
}

func (a ActiveAction) Available() bool {
	return a.ActionRef != "" && a.PayloadDigest != "" && a.ActionClass != ""
}

type ExtractRequest struct {
	SourceRef     string `json:"sourceRef"`
	PayloadDigest string `json:"payloadDigest"`
	Profile       string `json:"profile"`
}

type ExtractResult struct {
	Fields       map[string]string `json:"fields"`
	Confidence   string            `json:"confidence"`
	EvidenceRefs []string          `json:"evidenceRefs,omitempty"`
}

type ExtractResponse struct {
	ActionRef     string            `json:"actionRef"`
	PayloadDigest string            `json:"payloadDigest"`
	Profile       string            `json:"profile"`
	Fields        map[string]string `json:"fields"`
	Confidence    string            `json:"confidence"`
	EvidenceRefs  []string          `json:"evidenceRefs,omitempty"`
}

type Extractor interface {
	Extract(ctx context.Context, active ActiveAction, request ExtractRequest) (ExtractResult, error)
}

type DisabledExtractor struct{}

func (DisabledExtractor) Extract(context.Context, ActiveAction, ExtractRequest) (ExtractResult, error) {
	return ExtractResult{}, ErrExtractorUnavailable
}

type EffectIntent struct {
	ActionClass   string `json:"actionClass"`
	Connector     string `json:"connector"`
	Scope         string `json:"scope"`
	PayloadDigest string `json:"payloadDigest"`
	BodyDigest    string `json:"bodyDigest,omitempty"`
}

type EffectDecision struct {
	ActionRef     string `json:"actionRef"`
	Decision      string `json:"decision"`
	RuleFired     Rule   `json:"ruleFired"`
	Reason        string `json:"reason"`
	AuditRef      string `json:"auditRef"`
	EffectRef     string `json:"effectRef,omitempty"`
	EscalationRef string `json:"escalationRef,omitempty"`
}

type Rule struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type EffectMediator interface {
	RequestEffect(ctx context.Context, active ActiveAction, intent EffectIntent) (EffectDecision, error)
}

type DenyingEffectMediator struct{}

func (DenyingEffectMediator) RequestEffect(_ context.Context, active ActiveAction, _ EffectIntent) (EffectDecision, error) {
	return EffectDecision{
		ActionRef: active.ActionRef,
		Decision:  "deny",
		RuleFired: Rule{ID: "R-PEP-ADJUDICATION-NOT-CONFIGURED", Version: 1},
		Reason:    "policy adjudication is not configured in this PEP build; fail closed",
	}, nil
}

type LoopbackConfig struct {
	ActiveAction ActiveAction
	Extractor    Extractor
	Effects      EffectMediator
}

func NewLoopbackHandler(config LoopbackConfig) http.Handler {
	server := loopbackServer{
		active:    config.ActiveAction,
		extractor: config.Extractor,
		effects:   config.Effects,
	}
	if server.extractor == nil {
		server.extractor = DisabledExtractor{}
	}
	if server.effects == nil {
		server.effects = DenyingEffectMediator{}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/healthz", server.healthz)
	mux.HandleFunc("/v1/extract", server.extract)
	mux.HandleFunc("/v1/effects", server.effectsRequest)
	return mux
}

type loopbackServer struct {
	active    ActiveAction
	extractor Extractor
	effects   EffectMediator
}

func (s loopbackServer) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"component": "gear-pep", "status": "ok", "listen": LoopbackListenAddress})
}

func (s loopbackServer) extract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request ExtractRequest
	if err := decodeStrict(r, &request); err != nil {
		http.Error(w, "invalid extract request", http.StatusBadRequest)
		return
	}
	if err := validateExtractRequest(request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.validateActive(request.PayloadDigest, ""); err != nil {
		writeValidationError(w, err)
		return
	}

	result, err := s.extractor.Extract(r.Context(), s.active, request)
	if err != nil {
		if errors.Is(err, ErrExtractorUnavailable) {
			http.Error(w, "extractor unavailable", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "extract failed", http.StatusBadGateway)
		return
	}

	writeJSON(w, http.StatusOK, ExtractResponse{
		ActionRef:     s.active.ActionRef,
		PayloadDigest: request.PayloadDigest,
		Profile:       request.Profile,
		Fields:        result.Fields,
		Confidence:    result.Confidence,
		EvidenceRefs:  result.EvidenceRefs,
	})
}

func (s loopbackServer) effectsRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var intent EffectIntent
	if err := decodeStrict(r, &intent); err != nil {
		http.Error(w, "invalid effect request", http.StatusBadRequest)
		return
	}
	if err := validateEffectIntent(intent); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.validateActive(intent.PayloadDigest, intent.ActionClass); err != nil {
		writeValidationError(w, err)
		return
	}

	decision, err := s.effects.RequestEffect(r.Context(), s.active, intent)
	if err != nil {
		writeJSON(w, http.StatusOK, EffectDecision{
			ActionRef: s.active.ActionRef,
			Decision:  "deny",
			RuleFired: Rule{ID: "R-PEP-EFFECT-MEDIATOR-UNAVAILABLE", Version: 1},
			Reason:    "effect mediator unavailable; fail closed",
		})
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (s loopbackServer) validateActive(payloadDigest, actionClass string) error {
	if !s.active.Available() {
		return ErrActiveActionUnavailable
	}
	if payloadDigest != s.active.PayloadDigest {
		return fmt.Errorf("%w: payloadDigest mismatch", ErrRequestRejected)
	}
	if actionClass != "" && actionClass != s.active.ActionClass {
		return fmt.Errorf("%w: actionClass mismatch", ErrRequestRejected)
	}
	return nil
}

func validateExtractRequest(request ExtractRequest) error {
	if request.SourceRef == "" {
		return fmt.Errorf("%w: sourceRef is required", ErrRequestRejected)
	}
	if request.PayloadDigest == "" {
		return fmt.Errorf("%w: payloadDigest is required", ErrRequestRejected)
	}
	if !isSHA256Ref(request.PayloadDigest) {
		return fmt.Errorf("%w: payloadDigest must be a sha256 reference", ErrRequestRejected)
	}
	if request.Profile == "" {
		return fmt.Errorf("%w: profile is required", ErrRequestRejected)
	}
	return nil
}

func validateEffectIntent(intent EffectIntent) error {
	if intent.ActionClass == "" {
		return fmt.Errorf("%w: actionClass is required", ErrRequestRejected)
	}
	if intent.Connector == "" {
		return fmt.Errorf("%w: connector is required", ErrRequestRejected)
	}
	if intent.Scope == "" {
		return fmt.Errorf("%w: scope is required", ErrRequestRejected)
	}
	if intent.PayloadDigest == "" {
		return fmt.Errorf("%w: payloadDigest is required", ErrRequestRejected)
	}
	if !isSHA256Ref(intent.PayloadDigest) {
		return fmt.Errorf("%w: payloadDigest must be a sha256 reference", ErrRequestRejected)
	}
	if intent.BodyDigest != "" && !isSHA256Ref(intent.BodyDigest) {
		return fmt.Errorf("%w: bodyDigest must be a sha256 reference", ErrRequestRejected)
	}
	return nil
}

func writeValidationError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrActiveActionUnavailable) {
		http.Error(w, "active governed action unavailable", http.StatusServiceUnavailable)
		return
	}
	if errors.Is(err, ErrRequestRejected) {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Error(w, "request rejected", http.StatusForbidden)
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

func ValidateLoopbackListenAddress(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host != "127.0.0.1" || port != "9191" {
		return fmt.Errorf("gear-pep must listen on %s, got %s", LoopbackListenAddress, addr)
	}
	return nil
}

func ActiveActionFromEnv(lookup func(string) (string, bool)) (ActiveAction, error) {
	mandateVersion := 0
	if value, ok := lookup("GEAR_MANDATE_VERSION"); ok && strings.TrimSpace(value) != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return ActiveAction{}, fmt.Errorf("invalid GEAR_MANDATE_VERSION: %w", err)
		}
		mandateVersion = parsed
	}
	counters, err := countersFromEnv(lookup)
	if err != nil {
		return ActiveAction{}, err
	}

	return ActiveAction{
		ActionRef:      env(lookup, "GEAR_ACTION_REF"),
		ActionClass:    env(lookup, "GEAR_ACTION_CLASS"),
		AbilityRef:     env(lookup, "GEAR_ABILITY_REF"),
		AbilityVersion: env(lookup, "GEAR_ABILITY_VERSION"),
		MandateRef:     env(lookup, "GEAR_MANDATE_REF"),
		MandateVersion: mandateVersion,
		SubjectRef:     env(lookup, "GEAR_SUBJECT_REF"),
		DataClasses:    csvEnv(lookup, "GEAR_DATA_CLASSES"),
		Confidence:     env(lookup, "GEAR_CONFIDENCE"),
		Reversibility:  env(lookup, "GEAR_REVERSIBILITY"),
		Counters:       counters,
		PayloadDigest:  env(lookup, "GEAR_PAYLOAD_DIGEST"),
	}, nil
}

func countersFromEnv(lookup func(string) (string, bool)) (map[string]int, error) {
	counters := map[string]int{}
	if value := env(lookup, "GEAR_COUNTERS"); value != "" {
		if err := json.Unmarshal([]byte(value), &counters); err != nil {
			return nil, fmt.Errorf("invalid GEAR_COUNTERS: %w", err)
		}
	}
	for _, item := range []struct {
		key  string
		name string
	}{
		{key: "GEAR_COUNTER_DAILY_ACTIONS", name: "dailyActions"},
		{key: "GEAR_COUNTER_PER_SUBJECT", name: "perSubject"},
	} {
		value := env(lookup, item.key)
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", item.key, err)
		}
		counters[item.name] = parsed
	}
	return counters, nil
}

func env(lookup func(string) (string, bool), key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func csvEnv(lookup func(string) (string, bool), key string) []string {
	value := env(lookup, key)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func NewLoopbackServer(addr string, handler http.Handler) (*http.Server, error) {
	if err := ValidateLoopbackListenAddress(addr); err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}, nil
}

func isSHA256Ref(value string) bool {
	if !strings.HasPrefix(value, "sha256:") {
		return false
	}
	digest := strings.TrimPrefix(value, "sha256:")
	if len(digest) == 64 {
		for _, r := range digest {
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
		return true
	}
	return digest != ""
}
