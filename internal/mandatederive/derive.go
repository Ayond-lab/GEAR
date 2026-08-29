package mandatederive

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
	"regexp"
	"strings"
	"time"

	gearv1 "gear/api/v1"
	"gear/internal/chain"
	"gear/internal/legality"
	"gear/internal/mandatesign"
	"gear/internal/subsume"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ErrRefusalAuditUnavailable = errors.New("mandate-refused audit unavailable")

type AuditAppender interface {
	Append(ctx context.Context, entry chain.Entry) (chain.Entry, error)
}

type Request struct {
	Namespace           string             `json:"namespace,omitempty"`
	Name                string             `json:"name,omitempty"`
	MandateID           string             `json:"mandateId"`
	Version             int                `json:"version"`
	AbilityRef          string             `json:"abilityRef"`
	Ability             gearv1.AbilitySpec `json:"ability"`
	PurposeStatement    string             `json:"purposeStatement"`
	LegalBasis          string             `json:"legalBasis,omitempty"`
	ExpiresAt           string             `json:"expiresAt,omitempty"`
	OperatorResponseRef string             `json:"operatorResponseRef,omitempty"`
}

type Response struct {
	Outcome       string          `json:"outcome"`
	Mandate       *gearv1.Mandate `json:"mandate,omitempty"`
	Clauses       []Clause        `json:"clauses,omitempty"`
	Refusal       *Refusal        `json:"refusal,omitempty"`
	AuditRef      string          `json:"auditRef,omitempty"`
	Subsumed      bool            `json:"subsumed,omitempty"`
	Signed        bool            `json:"signed,omitempty"`
	Clarification []string        `json:"clarification,omitempty"`
}

type Refusal struct {
	RuleID              string   `json:"ruleId"`
	Criterion           string   `json:"criterion"`
	Verb                string   `json:"verb"`
	Reason              string   `json:"reason"`
	PurposeDigest       string   `json:"purposeDigest"`
	OperatorResponseRef string   `json:"operatorResponseRef,omitempty"`
	Alternatives        []string `json:"alternatives"`
}

type Deriver struct {
	Audit      AuditAppender
	SigningKey *ecdsa.PrivateKey
	Now        func() time.Time
}

func NewDeriver(audit AuditAppender) Deriver {
	return Deriver{Audit: audit, SigningKey: mandatesign.DevelopmentPrivateKey(), Now: time.Now}
}

func (d Deriver) Derive(ctx context.Context, request Request) (Response, error) {
	request = normaliseRequest(request)
	eval := legality.EvaluatePurpose(request.PurposeStatement)
	if eval.Decision == legality.DecisionRefuse {
		return d.refuse(ctx, request, eval)
	}

	mandate, clauses, err := d.signedNarrowedMandate(request)
	if err != nil {
		return Response{}, err
	}
	return Response{
		Outcome:  "signed",
		Mandate:  mandate,
		Clauses:  clauses,
		Subsumed: true,
		Signed:   true,
	}, nil
}

func (d Deriver) refuse(ctx context.Context, request Request, eval legality.Evaluation) (Response, error) {
	refusal := Refusal{
		RuleID:              eval.RuleID,
		Criterion:           eval.Criterion,
		Verb:                eval.Verb,
		Reason:              eval.Reason,
		PurposeDigest:       digestBytes([]byte(request.PurposeStatement)),
		OperatorResponseRef: request.OperatorResponseRef,
		Alternatives:        append([]string(nil), eval.Alternatives...),
	}
	entry := refusalAuditEntry(request, refusal, d.now())
	if d.Audit == nil {
		return Response{}, ErrRefusalAuditUnavailable
	}
	stored, err := d.Audit.Append(ctx, entry)
	if err != nil {
		return Response{}, fmt.Errorf("%w: %v", ErrRefusalAuditUnavailable, err)
	}
	auditRef := chain.Ref(stored.Seq)
	return Response{
		Outcome:       "refused",
		Clauses:       RefusalClauses(eval),
		Refusal:       &refusal,
		AuditRef:      auditRef,
		Clarification: append([]string(nil), eval.Alternatives...),
	}, nil
}

func (d Deriver) signedNarrowedMandate(request Request) (*gearv1.Mandate, []Clause, error) {
	mandate, clauses, err := NarrowedCVMandate(request)
	if err != nil {
		return nil, nil, err
	}
	if result := subsume.Check(request.Ability, mandate.Spec); !result.OK() {
		return nil, nil, result.Error()
	}
	key := d.SigningKey
	if key == nil {
		key = mandatesign.DevelopmentPrivateKey()
	}
	signature, err := mandatesign.Sign(mandate.Spec, key)
	if err != nil {
		return nil, nil, err
	}
	mandate.Spec.Signature = signature
	return mandate, clauses, nil
}

func NarrowedCVMandate(request Request) (*gearv1.Mandate, []Clause, error) {
	request = normaliseRequest(request)
	if request.MandateID == "" {
		return nil, nil, errors.New("mandateId is required")
	}
	if request.AbilityRef == "" {
		return nil, nil, errors.New("abilityRef is required")
	}
	if request.Ability.Version == "" {
		return nil, nil, errors.New("ability.version is required")
	}
	if request.PurposeStatement == "" {
		return nil, nil, errors.New("purposeStatement is required")
	}
	expiresAt, err := parseExpiry(request.ExpiresAt)
	if err != nil {
		return nil, nil, err
	}

	mandate := &gearv1.Mandate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gearv1.GroupVersion.String(),
			Kind:       "Mandate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Name,
			Namespace: request.Namespace,
		},
		Spec: gearv1.MandateSpec{
			MandateID:        request.MandateID,
			Version:          request.Version,
			AbilityRef:       request.AbilityRef,
			AbilityVersion:   request.Ability.Version,
			PurposeStatement: request.PurposeStatement,
			LegalBasis:       request.LegalBasis,
			Sources:          sourcesFromTriggers(request.Ability.DeclaredTriggers),
			ConnectorGrants:  append([]gearv1.ConnectorScope(nil), request.Ability.ConnectorScopes...),
			ActionGrants:     cvActionGrants(request.Ability.ActionClasses),
			Caps:             gearv1.Caps{DailyActions: narrowedDailyCap(request.Ability.Ceilings.DailyActions)},
			Thresholds:       map[string]string{"extractionConfidence": "0.70"},
			Approvers:        []gearv1.Approver{{ID: "hiring-manager-1", Name: "Hiring Manager"}},
			Egress:           []gearv1.EgressRule{},
			ExpiresAt:        metav1.NewTime(expiresAt),
			CredentialRef: corev1.SecretReference{
				Name:      k8sName(request.MandateID) + "-credential",
				Namespace: request.Namespace,
			},
		},
	}
	return mandate, NarrowedCVClauses(), nil
}

func NewHandler(deriver Deriver) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"component": "gear-mandate", "status": "ok"})
	})
	mux.HandleFunc("/v1/derive", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var request Request
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "invalid derive request", http.StatusBadRequest)
			return
		}
		response, err := deriver.Derive(r.Context(), request)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, ErrRefusalAuditUnavailable) {
				status = http.StatusServiceUnavailable
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, http.StatusOK, response)
	})
	return mux
}

func refusalAuditEntry(request Request, refusal Refusal, now time.Time) chain.Entry {
	dataAccessed := []string{refusal.PurposeDigest}
	if request.OperatorResponseRef != "" {
		dataAccessed = append(dataAccessed, request.OperatorResponseRef)
	}
	return chain.Entry{
		TS:           now.UTC().Format(time.RFC3339Nano),
		Type:         "mandate-refused",
		ActionRef:    "mandate-" + shortDigest([]byte(request.MandateID+"|"+request.AbilityRef+"|"+refusal.PurposeDigest)),
		Actor:        "gear-mandate",
		Mandate:      fmt.Sprintf("%s:%d", request.MandateID, request.Version),
		Rule:         refusal.RuleID + ":1",
		Decision:     "deny",
		InputsDigest: digestRefusalInput(request, refusal),
		Model:        "none",
		DataAccessed: dataAccessed,
	}
}

func digestRefusalInput(request Request, refusal Refusal) string {
	input := struct {
		MandateID           string `json:"mandateId"`
		Version             int    `json:"version"`
		AbilityRef          string `json:"abilityRef"`
		AbilityVersion      string `json:"abilityVersion"`
		PurposeDigest       string `json:"purposeDigest"`
		OperatorResponseRef string `json:"operatorResponseRef,omitempty"`
		RuleID              string `json:"ruleId"`
	}{
		MandateID:           request.MandateID,
		Version:             request.Version,
		AbilityRef:          request.AbilityRef,
		AbilityVersion:      request.Ability.Version,
		PurposeDigest:       refusal.PurposeDigest,
		OperatorResponseRef: request.OperatorResponseRef,
		RuleID:              refusal.RuleID,
	}
	data, err := json.Marshal(input)
	if err != nil {
		return digestBytes([]byte(fmt.Sprintf("%#v", input)))
	}
	return digestBytes(bytes.TrimSpace(data))
}

func normaliseRequest(request Request) Request {
	if request.Namespace == "" {
		request.Namespace = "gear-lab"
	}
	if request.Name == "" && request.MandateID != "" {
		request.Name = k8sName(request.MandateID)
	}
	if request.Version == 0 {
		request.Version = 1
	}
	if request.LegalBasis == "" {
		request.LegalBasis = "Right-to-work verification"
	}
	if request.ExpiresAt == "" {
		request.ExpiresAt = "2027-02-01T00:00:00Z"
	}
	return request
}

func parseExpiry(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid expiresAt: %w", err)
	}
	return parsed.UTC(), nil
}

func sourcesFromTriggers(triggers []gearv1.TriggerDecl) []gearv1.Source {
	sources := make([]gearv1.Source, 0, len(triggers))
	for _, trigger := range triggers {
		sources = append(sources, gearv1.Source{Type: trigger.Type, ID: trigger.ID})
	}
	return sources
}

func cvActionGrants(actions []string) []gearv1.ActionGrant {
	actionSet := map[string]bool{}
	for _, action := range actions {
		actionSet[action] = true
	}
	var grants []gearv1.ActionGrant
	for _, item := range []gearv1.ActionGrant{
		{Class: "RECORD_ANNOTATE", Disposition: "permit"},
		{Class: "RECORD_MODIFY", Disposition: "escalate"},
		{Class: "CANDIDATE_RANK", Disposition: "forbid"},
		{Class: "OUTBOUND_COMMS", Disposition: "forbid"},
	} {
		if actionSet[item.Class] {
			grants = append(grants, item)
		}
	}
	return grants
}

func narrowedDailyCap(ceiling int) int {
	if ceiling > 0 && ceiling < 50 {
		return ceiling
	}
	return 50
}

func k8sName(value string) string {
	lowered := strings.ToLower(value)
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	name := strings.Trim(re.ReplaceAllString(lowered, "-"), "-")
	if name == "" {
		return "mandate"
	}
	return name
}

func (d Deriver) now() time.Time {
	if d.Now == nil {
		return time.Now()
	}
	return d.Now()
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func shortDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
