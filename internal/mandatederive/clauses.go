package mandatederive

import "gear/internal/legality"

type Clause struct {
	ID     string
	Text   string
	Reason string
}

func RefusalClauses(eval legality.Evaluation) []Clause {
	if eval.Decision != legality.DecisionRefuse {
		return nil
	}
	return []Clause{
		{
			ID:     "D1",
			Text:   "Forbid CANDIDATE_RANK because legality gate refused protected-attribute selective use.",
			Reason: eval.Reason,
		},
		{
			ID:     "D2",
			Text:   "Forbid OUTBOUND_COMMS because the purpose does not imply candidate contact.",
			Reason: "purpose does not imply candidate contact",
		},
	}
}

