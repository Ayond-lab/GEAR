package policy

import "testing"

func TestDecideDeniesForbiddenCandidateRank(t *testing.T) {
	result := Decide(validInput("CANDIDATE_RANK", "0.91"), cvRuntimePolicy())

	if result.Decision != Deny {
		t.Fatalf("expected deny, got %q", result.Decision)
	}
	if result.RuleFired.ID != "D1" {
		t.Fatalf("expected D1, got %q", result.RuleFired.ID)
	}
}

func TestDecideEscalatesLowConfidence(t *testing.T) {
	result := Decide(validInput("RECORD_ANNOTATE", "0.42"), cvRuntimePolicy())

	if result.Decision != Escalate {
		t.Fatalf("expected escalate, got %q", result.Decision)
	}
}

func TestDecideAuthorisesPermittedAction(t *testing.T) {
	result := Decide(validInput("RECORD_ANNOTATE", "0.84"), cvRuntimePolicy())

	if result.Decision != Authorise {
		t.Fatalf("expected authorise, got %q", result.Decision)
	}
}

func validInput(actionClass, confidence string) DecisionInput {
	return DecisionInput{
		ActionClass:    actionClass,
		AbilityRef:     "cv-screen",
		AbilityVersion: "0.3.0",
		MandateRef:     "MND-2026-021",
		MandateVersion: 2,
		Confidence:     confidence,
		DataClasses:    []string{"personal", "protected-employment"},
		Reversibility:  "reversible",
		Counters:       map[string]int{"dailyActions": 12, "perSubject": 1},
		PayloadDigest:  "sha256:abc",
	}
}

func cvRuntimePolicy() RuntimePolicy {
	return RuntimePolicy{
		ActionDispositions: map[string]Disposition{
			"RECORD_ANNOTATE": {Value: "permit", Clause: "P1"},
			"RECORD_MODIFY":   {Value: "escalate", Clause: "E1"},
			"CANDIDATE_RANK":  {Value: "forbid", Clause: "D1"},
			"OUTBOUND_COMMS":  {Value: "forbid", Clause: "D2"},
		},
		ConfidenceThreshold: "0.70",
		ApproverCount:       1,
		TokenScopes: map[string]EffectScope{
			"RECORD_ANNOTATE": {Connector: "candidate-record", Scope: "write"},
		},
	}
}
