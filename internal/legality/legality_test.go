package legality

import "testing"

func TestEvaluatePurposeRefusesProtectedSelectiveUse(t *testing.T) {
	result := EvaluatePurpose("Check the CVs, select the candidates who are not citizens of the EEA.")

	if result.Decision != DecisionRefuse {
		t.Fatalf("expected refusal, got %q", result.Decision)
	}
	if result.Criterion != "citizenship" {
		t.Fatalf("expected citizenship criterion, got %q", result.Criterion)
	}
	if result.Verb != "select" {
		t.Fatalf("expected select verb, got %q", result.Verb)
	}
	if len(result.Alternatives) != 2 {
		t.Fatalf("expected two lawful alternatives, got %d", len(result.Alternatives))
	}
}

func TestEvaluatePurposeAllowsObservationalWorkAuthorisation(t *testing.T) {
	result := EvaluatePurpose("Identify candidates who will require work authorisation, for planning.")

	if result.Decision != DecisionAllow {
		t.Fatalf("expected allow, got %q", result.Decision)
	}
}

func TestEvaluatePurposeDoesNotRefuseProtectedObservation(t *testing.T) {
	result := EvaluatePurpose("Extract citizenship status for human review.")

	if result.Decision != DecisionAllow {
		t.Fatalf("expected allow for observational verb, got %q", result.Decision)
	}
}

