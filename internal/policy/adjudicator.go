package policy

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gear/internal/chain"
	"gear/internal/exectoken"
)

type AuditAppender interface {
	Append(ctx context.Context, entry chain.Entry) (chain.Entry, error)
}

type Adjudicator struct {
	runtime  RuntimePolicy
	audit    AuditAppender
	now      func() time.Time
	tokenKey *ecdsa.PrivateKey
	tokenTTL time.Duration
}

func NewAdjudicator(runtime RuntimePolicy, audit AuditAppender) *Adjudicator {
	return &Adjudicator{
		runtime:  runtime,
		audit:    audit,
		now:      time.Now,
		tokenKey: exectoken.DevelopmentPrivateKey(),
		tokenTTL: 30 * time.Second,
	}
}

func (a *Adjudicator) SetExecutionTokenPrivateKey(key *ecdsa.PrivateKey) {
	a.tokenKey = key
}

func (a *Adjudicator) Adjudicate(ctx context.Context, raw []byte) DecisionResult {
	now := time.Now
	if a.now != nil {
		now = a.now
	}

	input, err := DecodeExactDecisionInput(raw)
	result := DecisionResult{}
	actionRef := "ga-invalid-" + shortDigest(raw)
	inputsDigest := digestBytes(raw)
	mandateRef := ""
	dataAccessed := []string(nil)
	if err != nil {
		result = DecisionResult{
			Decision:  Deny,
			RuleFired: RuleRef{ID: "R-INPUT-INVALID", Version: 1},
			Reason:    "invalid decision input",
		}
	} else {
		result = Decide(input, a.runtime)
		actionRef = ActionRef(input)
		inputsDigest = InputDigest(input)
		mandateRef = fmt.Sprintf("%s:%d", input.MandateRef, input.MandateVersion)
		dataAccessed = []string{input.PayloadDigest}
		if result.Decision == Authorise {
			result = a.attachExecutionToken(result, actionRef, input, now())
		}
	}

	entry := chain.Entry{
		TS:           now().UTC().Format(time.RFC3339Nano),
		Type:         "decision",
		ActionRef:    actionRef,
		Actor:        "gear-policy",
		Mandate:      mandateRef,
		Rule:         fmt.Sprintf("%s:%d", result.RuleFired.ID, result.RuleFired.Version),
		Decision:     string(result.Decision),
		InputsDigest: inputsDigest,
		Model:        "none",
		DataAccessed: dataAccessed,
	}

	var (
		stored    chain.Entry
		appendErr error
	)
	if a.audit == nil {
		appendErr = errors.New("audit appender is nil")
	} else {
		stored, appendErr = a.audit.Append(ctx, entry)
	}
	if appendErr != nil {
		return DecisionResult{
			Decision:  Deny,
			RuleFired: RuleRef{ID: "R-AUDIT-UNAVAILABLE", Version: 1},
			Reason:    "audit unavailable; fail closed",
		}
	}
	result.AuditRef = chain.Ref(stored.Seq)

	switch result.Decision {
	case Escalate:
		escalationRef := "esc-" + shortDigest([]byte(actionRef+result.AuditRef))
		result.EscalationRef = &escalationRef
	}

	return result
}

func (a *Adjudicator) attachExecutionToken(result DecisionResult, actionRef string, input DecisionInput, now time.Time) DecisionResult {
	scope, ok := a.runtime.TokenScopes[input.ActionClass]
	if !ok || scope.Connector == "" || scope.Scope == "" {
		return DecisionResult{
			Decision:  Deny,
			RuleFired: RuleRef{ID: "R-TOKEN-SCOPE-MISSING", Version: 1},
			Reason:    "execution token scope is unavailable; fail closed",
		}
	}
	ttl := a.tokenTTL
	if ttl <= 0 || ttl > 30*time.Second {
		ttl = 30 * time.Second
	}
	claims := exectoken.Claims{
		ActionRef:      actionRef,
		Connector:      scope.Connector,
		Scope:          scope.Scope,
		PayloadDigest:  input.PayloadDigest,
		MandateVersion: input.MandateVersion,
		Audience:       "gear-pep",
		ExpiresAt:      now.Add(ttl).Unix(),
		JTI:            shortDigest([]byte(fmt.Sprintf("%s|%s|%d|%d", actionRef, input.PayloadDigest, input.MandateVersion, now.UnixNano()))),
	}
	token, err := exectoken.SignES256(a.tokenKey, claims)
	if err != nil {
		return DecisionResult{
			Decision:  Deny,
			RuleFired: RuleRef{ID: "R-TOKEN-ISSUE", Version: 1},
			Reason:    "execution token could not be issued; fail closed",
		}
	}
	result.Token = &token
	return result
}

func ActionRef(input DecisionInput) string {
	return "ga-" + shortDigest([]byte(fmt.Sprintf(
		"%s|%s|%s|%s|%d|%s",
		input.ActionClass,
		input.AbilityRef,
		input.AbilityVersion,
		input.MandateRef,
		input.MandateVersion,
		input.PayloadDigest,
	)))
}

func InputDigest(input DecisionInput) string {
	data, err := json.Marshal(input)
	if err != nil {
		return digestBytes([]byte(fmt.Sprintf("%#v", input)))
	}
	return digestBytes(data)
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func shortDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

type HTTPAuditClient struct {
	BaseURL string
	Client  *http.Client
}

func NewHTTPAuditClient(baseURL string) *HTTPAuditClient {
	return &HTTPAuditClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *HTTPAuditClient) Append(ctx context.Context, entry chain.Entry) (chain.Entry, error) {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return chain.Entry{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/entries", bytes.NewReader(data))
	if err != nil {
		return chain.Entry{}, err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return chain.Entry{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return chain.Entry{}, fmt.Errorf("audit append status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var response struct {
		Entry chain.Entry `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return chain.Entry{}, err
	}
	return response.Entry, nil
}

func NewHandler(adjudicator *Adjudicator) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", NewHealthHandler())
	mux.HandleFunc("/v1/adjudicate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read request failed", http.StatusBadRequest)
			return
		}
		response := adjudicator.Adjudicate(r.Context(), raw)
		writeJSON(w, http.StatusOK, response)
	})
	return mux
}

func NewHealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"component": "gear-policy", "status": "ok"})
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
