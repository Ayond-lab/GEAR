package mandatederive

import "gear/internal/legality"

type Clause struct {
	ID     string
	Text   string
	Reason string
}

func NarrowedCVClauses() []Clause {
	return []Clause{
		{
			ID:     "D1",
			Text:   "Forbid CANDIDATE_RANK because legality gate refused protected-attribute selective use.",
			Reason: "protected-attribute selective use is outside this mandate",
		},
		{
			ID:     "D2",
			Text:   "Forbid OUTBOUND_COMMS because the purpose does not imply candidate contact.",
			Reason: "purpose does not imply candidate contact",
		},
	}
}

func RefusalClauses(eval legality.Evaluation) []Clause {
	if eval.Decision != legality.DecisionRefuse {
		return nil
	}
	clauses := NarrowedCVClauses()
	clauses[0].Reason = eval.Reason
	return clauses
}
