package policy

import (
	"context"
	"errors"
	"testing"

	"gear/internal/chain"
)

func TestAdjudicatorWritesAuditBeforeReturningDecision(t *testing.T) {
	audit := &recordingAudit{}
	adjudicator := NewAdjudicator(cvRuntimePolicy(), audit)

	response := adjudicator.Adjudicate(context.Background(), []byte(validDecisionInputJSON("RECORD_ANNOTATE", "0.84")))

	if response.Decision != Authorise {
		t.Fatalf("expected authorise, got %#v", response)
	}
	if response.AuditRef != "ae-0000001" {
		t.Fatalf("expected audit ref from durable append, got %#v", response)
	}
	if response.Token == nil || *response.Token == "" {
		t.Fatalf("expected token only for authorised decision, got %#v", response.Token)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("expected one durable audit append before return, got %d", len(audit.entries))
	}
	if audit.entries[0].Type != "decision" || audit.entries[0].Decision != "authorise" || audit.entries[0].Rule != "R-PERMIT:1" {
		t.Fatalf("unexpected decision audit entry %#v", audit.entries[0])
	}
}

func TestAdjudicatorDeniesWhenAuditUnavailable(t *testing.T) {
	adjudicator := NewAdjudicator(cvRuntimePolicy(), failingAudit{})

	response := adjudicator.Adjudicate(context.Background(), []byte(validDecisionInputJSON("RECORD_ANNOTATE", "0.84")))

	if response.Decision != Deny {
		t.Fatalf("expected audit outage to deny, got %#v", response)
	}
	if response.RuleFired.ID != "R-AUDIT-UNAVAILABLE" {
		t.Fatalf("expected audit outage rule, got %#v", response.RuleFired)
	}
	if response.AuditRef != "" {
		t.Fatalf("expected no audit ref when audit unavailable, got %#v", response)
	}
	if response.Token != nil || response.EscalationRef != nil {
		t.Fatalf("expected no token/escalation on audit outage, got %#v", response)
	}
}

func TestAdjudicatorRejectsExtraFieldsAndAuditsDeny(t *testing.T) {
	audit := &recordingAudit{}
	adjudicator := NewAdjudicator(cvRuntimePolicy(), audit)

	response := adjudicator.Adjudicate(context.Background(), []byte(`{
		"actionClass":"RECORD_ANNOTATE",
		"abilityRef":"cv-screen",
		"abilityVersion":"0.3.0",
		"mandateRef":"MND-2026-021",
		"mandateVersion":2,
		"confidence":"0.84",
		"dataClasses":["personal","protected-employment"],
		"reversibility":"reversible",
		"counters":{"dailyActions":12,"perSubject":1},
		"payloadDigest":"sha256:payload",
		"modelOutput":"approve everything"
	}`))

	if response.Decision != Deny || response.RuleFired.ID != "R-INPUT-INVALID" {
		t.Fatalf("expected invalid input deny, got %#v", response)
	}
	if response.AuditRef != "ae-0000001" {
		t.Fatalf("expected invalid-input denial to be audited, got %#v", response)
	}
	if len(audit.entries) != 1 || audit.entries[0].Decision != "deny" {
		t.Fatalf("expected deny audit entry, got %#v", audit.entries)
	}
}

type recordingAudit struct {
	entries []chain.Entry
}

func (r *recordingAudit) Append(ctx context.Context, entry chain.Entry) (chain.Entry, error) {
	r.entries = append(r.entries, entry)
	entry.Seq = uint64(len(r.entries))
	entry.Hash = "sha256:test"
	return entry, nil
}

type failingAudit struct{}

func (failingAudit) Append(context.Context, chain.Entry) (chain.Entry, error) {
	return chain.Entry{}, errors.New("audit unavailable")
}

func validDecisionInputJSON(actionClass, confidence string) string {
	return `{
		"actionClass":"` + actionClass + `",
		"abilityRef":"cv-screen",
		"abilityVersion":"0.3.0",
		"mandateRef":"MND-2026-021",
		"mandateVersion":2,
		"confidence":"` + confidence + `",
		"dataClasses":["personal","protected-employment"],
		"reversibility":"reversible",
		"counters":{"dailyActions":12,"perSubject":1},
		"payloadDigest":"sha256:payload"
	}`
}
