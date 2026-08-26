package mandatederive

import (
	"testing"

	"gear/internal/legality"
)

func TestRefusalClausesForProtectedSelectiveUse(t *testing.T) {
	eval := legality.EvaluatePurpose("Check the CVs, select the candidates who are not citizens of the EEA.")
	clauses := RefusalClauses(eval)

	if len(clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(clauses))
	}
	if clauses[0].ID != "D1" || clauses[1].ID != "D2" {
		t.Fatalf("unexpected clause IDs: %#v", clauses)
	}
}

