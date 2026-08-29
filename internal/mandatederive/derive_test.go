package mandatederive

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	gearv1 "gear/api/v1"
	"gear/internal/chain"
	"gear/internal/mandatesign"
)

func TestDeriveRefusesProtectedSelectivePurposeAndAudits(t *testing.T) {
	audit := &recordingAudit{}
	deriver := NewDeriver(audit)
	deriver.Now = func() time.Time { return time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC) }

	response, err := deriver.Derive(context.Background(), Request{
		MandateID:           "MND-A1-REFUSED",
		Version:             1,
		AbilityRef:          "cv-screen",
		Ability:             cvScreenAbilitySpec(),
		PurposeStatement:    "Check the CVs, select the candidates who are not citizens of the EEA.",
		OperatorResponseRef: "sha256:operator-response",
	})
	if err != nil {
		t.Fatal(err)
	}

	if response.Outcome != "refused" || response.Mandate != nil || response.AuditRef != "ae-0000001" {
		t.Fatalf("unexpected refusal response %#v", response)
	}
	if response.Refusal == nil || response.Refusal.Criterion != "citizenship" || response.Refusal.Verb != "select" {
		t.Fatalf("expected citizenship/select refusal, got %#v", response.Refusal)
	}
	if len(response.Clarification) != 2 {
		t.Fatalf("expected two lawful alternatives, got %#v", response.Clarification)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("expected one audit entry, got %#v", audit.entries)
	}
	entry := audit.entries[0]
	if entry.Type != "mandate-refused" || entry.Actor != "gear-mandate" || entry.Decision != "deny" || entry.Rule != "LG-PROTECTED-SELECTIVE-01:1" {
		t.Fatalf("unexpected mandate-refused audit entry %#v", entry)
	}
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"Check the CVs", "citizens", "EEA"} {
		if strings.Contains(string(entryJSON), forbidden) {
			t.Fatalf("audit entry must not contain raw purpose text %q: %s", forbidden, entryJSON)
		}
	}
}

func TestDeriveSignsNarrowedMandateAfterSubsumption(t *testing.T) {
	response, err := NewDeriver(&recordingAudit{}).Derive(context.Background(), Request{
		Namespace:        "gear-lab",
		MandateID:        "MND-2026-021",
		Version:          2,
		AbilityRef:       "cv-screen",
		Ability:          cvScreenAbilitySpec(),
		PurposeStatement: "Identify candidates who will require work authorisation, for planning.",
		LegalBasis:       "Right-to-work verification",
		ExpiresAt:        "2027-02-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	if response.Outcome != "signed" || response.Mandate == nil || !response.Signed || !response.Subsumed {
		t.Fatalf("expected signed/subsumed mandate, got %#v", response)
	}
	if err := mandatesign.Verify(response.Mandate.Spec, mandatesign.DevelopmentPublicKey()); err != nil {
		t.Fatalf("expected signed mandate to verify, got %v", err)
	}
	grants := map[string]string{}
	for _, grant := range response.Mandate.Spec.ActionGrants {
		grants[grant.Class] = grant.Disposition
	}
	if grants["RECORD_ANNOTATE"] != "permit" || grants["RECORD_MODIFY"] != "escalate" || grants["CANDIDATE_RANK"] != "forbid" || grants["OUTBOUND_COMMS"] != "forbid" {
		t.Fatalf("unexpected action grants %#v", grants)
	}
	if len(response.Clauses) != 2 || response.Clauses[0].ID != "D1" || response.Clauses[1].ID != "D2" {
		t.Fatalf("expected D1/D2 clauses, got %#v", response.Clauses)
	}
}

func TestDeriveFailsClosedWhenRefusalAuditUnavailable(t *testing.T) {
	_, err := NewDeriver(failingAudit{}).Derive(context.Background(), Request{
		MandateID:        "MND-A1-REFUSED",
		Version:          1,
		AbilityRef:       "cv-screen",
		Ability:          cvScreenAbilitySpec(),
		PurposeStatement: "Check the CVs, select the candidates who are not citizens of the EEA.",
	})
	if !errors.Is(err, ErrRefusalAuditUnavailable) {
		t.Fatalf("expected refusal audit outage, got %v", err)
	}
}

type recordingAudit struct {
	entries []chain.Entry
}

func (r *recordingAudit) Append(_ context.Context, entry chain.Entry) (chain.Entry, error) {
	entry.Seq = uint64(len(r.entries) + 1)
	entry.Hash = "sha256:test"
	r.entries = append(r.entries, entry)
	return entry, nil
}

type failingAudit struct{}

func (failingAudit) Append(context.Context, chain.Entry) (chain.Entry, error) {
	return chain.Entry{}, errors.New("audit unavailable")
}

func cvScreenAbilitySpec() gearv1.AbilitySpec {
	return gearv1.AbilitySpec{
		Version:       "0.3.0",
		Certification: "certified",
		DeclaredTriggers: []gearv1.TriggerDecl{
			{Type: "folder", ID: "applications-inbox"},
		},
		ConnectorScopes: []gearv1.ConnectorScope{
			{Connector: "applications-store", Scope: "read"},
			{Connector: "candidate-record", Scope: "write"},
		},
		ActionClasses: []string{"RECORD_ANNOTATE", "RECORD_MODIFY", "CANDIDATE_RANK", "OUTBOUND_COMMS"},
		Reversibility: map[string]string{
			"RECORD_ANNOTATE": "reversible",
			"RECORD_MODIFY":   "reversible",
			"CANDIDATE_RANK":  "reversible",
			"OUTBOUND_COMMS":  "irreversible",
		},
		DataClasses: []string{"personal", "protected-employment"},
		Ceilings:    gearv1.Ceilings{DailyActions: 500},
	}
}
