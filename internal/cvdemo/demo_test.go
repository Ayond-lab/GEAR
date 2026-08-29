package cvdemo

import (
	"context"
	"testing"
	"time"

	"gear/internal/chain"
	"gear/internal/policy"
)

func TestGenerateApplicationsDistribution(t *testing.T) {
	applications := GenerateApplications()
	if len(applications) != 60 {
		t.Fatalf("expected 60 applications, got %d", len(applications))
	}
	counts := map[string]int{}
	injections := 0
	for _, application := range applications {
		counts[application.Status]++
		if application.InjectionCase {
			injections++
		}
		if application.SubjectRef == "" || application.PayloadDigest == "" || application.Salt == "" || application.ApplicationText == "" {
			t.Fatalf("fixture must include synthetic content and digest refs: %#v", application)
		}
	}
	expected := map[string]int{
		StatusEEANational:         34,
		StatusHoldsPermit:         11,
		StatusRequiresSponsorship: 12,
		StatusUnclear:             3,
	}
	for status, count := range expected {
		if counts[status] != count {
			t.Fatalf("status %q: expected %d, got %d", status, count, counts[status])
		}
	}
	if injections != 3 {
		t.Fatalf("expected 3 prompt-injection fixtures, got %d", injections)
	}
}

func TestBuildRecordAnnotationPlan(t *testing.T) {
	plan := BuildRecordAnnotationPlan(GenerateApplications())
	if plan.Applications != 60 || len(plan.Actions) != 48 || len(plan.NonMatches) != 12 {
		t.Fatalf("unexpected trigger plan summary: %#v", plan)
	}
	seen := map[string]bool{}
	for _, action := range plan.Actions {
		if action.Spec.ActionClass != "RECORD_ANNOTATE" {
			t.Fatalf("expected RECORD_ANNOTATE action, got %#v", action.Spec)
		}
		if seen[action.Spec.IdempotencyKey] {
			t.Fatalf("duplicate idempotency key %s", action.Spec.IdempotencyKey)
		}
		seen[action.Spec.IdempotencyKey] = true
	}
	for _, nonMatch := range plan.NonMatches {
		if nonMatch.ReasonCode != "requires-sponsorship-held-for-human-planning" {
			t.Fatalf("unexpected non-match reason %#v", nonMatch)
		}
	}
}

func TestRunCandidateRankDeny(t *testing.T) {
	result, err := RunCandidateRankDeny(context.Background(), fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Decision != string(policy.Deny) || result.Decision.RuleFired.ID != "D1" {
		t.Fatalf("expected deny/D1, got %#v", result.Decision)
	}
	if result.Decision.EffectRef != "" || result.GovernedStatus.ExecutionState != "refused" {
		t.Fatalf("candidate rank must produce no effect, got decision=%#v status=%#v", result.Decision, result.GovernedStatus)
	}
	if !result.ChainVerification.OK || len(result.AuditEntries) != 1 || result.AuditEntries[0].Type != "decision" {
		t.Fatalf("expected one verified decision entry, got verification=%#v entries=%#v", result.ChainVerification, result.AuditEntries)
	}
}

func TestRunRecordAnnotationPathCounts(t *testing.T) {
	result, err := RunRecordAnnotationPath(context.Background(), fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Applications != 60 || result.Summary.TriggeredActions != 48 || result.Summary.NonMatches != 12 {
		t.Fatalf("unexpected summary counts %#v", result.Summary)
	}
	if result.Summary.Authorised != 45 || result.Summary.Escalated != 3 || result.Summary.Denied != 0 || result.Summary.Effects != 45 {
		t.Fatalf("unexpected decision counts %#v", result.Summary)
	}
	if result.Summary.DecisionEntries != 48 || result.Summary.EffectEntries != 45 || result.Summary.NonMatchEntries != 12 {
		t.Fatalf("unexpected audit entry counts %#v", result.Summary)
	}
	if len(result.Escalations) != 3 || result.Summary.PendingEscalations != 3 {
		t.Fatalf("expected 3 pending escalation resources, got %d", len(result.Escalations))
	}
	if !result.ChainVerification.OK || len(result.EffectsWithoutDecision) != 0 {
		t.Fatalf("expected verified chain and no orphan effects, got verification=%#v orphan=%#v", result.ChainVerification, result.EffectsWithoutDecision)
	}
}

func TestRunPromptInjectionPairHoldsPolicyInputConstant(t *testing.T) {
	result, err := RunPromptInjectionPair(context.Background(), fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if result.CleanTextDigest == result.InjectedTextDigest {
		t.Fatal("expected clean and injected fixture digests to differ")
	}
	if result.CleanExtraction.WorkAuthorisationStatus != result.InjectedExtraction.WorkAuthorisationStatus || result.CleanExtraction.Confidence != result.InjectedExtraction.Confidence {
		t.Fatalf("expected extraction policy fields to remain stable, got clean=%#v injected=%#v", result.CleanExtraction, result.InjectedExtraction)
	}
	if !result.InjectedExtraction.PromptInjectionPresent {
		t.Fatal("expected injected fixture to be detected")
	}
	if result.CleanResponse.Decision != result.InjectedResponse.Decision || result.CleanResponse.RuleFired.ID != result.InjectedResponse.RuleFired.ID {
		t.Fatalf("expected same policy result for held-constant input, got clean=%#v injected=%#v", result.CleanResponse, result.InjectedResponse)
	}
	if result.InputDigest != policy.InputDigest(result.DecisionInput) {
		t.Fatalf("unexpected input digest %s", result.InputDigest)
	}
	if !result.ChainVerification.OK || len(result.AuditEntries) != 2 {
		t.Fatalf("expected two verified decision audit entries, got verification=%#v entries=%d", result.ChainVerification, len(result.AuditEntries))
	}
}

func TestEffectsWithoutDecisionsRequiresPrecedingDecision(t *testing.T) {
	missing := EffectsWithoutDecisions([]chain.Entry{
		{Seq: 1, Type: "effect", ActionRef: "ga-late-decision"},
		{Seq: 2, Type: "decision", ActionRef: "ga-late-decision"},
	})
	if len(missing) != 1 || missing[0] != "ga-late-decision" {
		t.Fatalf("expected late decision to be reported, got %#v", missing)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
}
