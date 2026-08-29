package policy

type Decision string

const (
	Authorise Decision = "authorise"
	Deny      Decision = "deny"
	Escalate  Decision = "escalate"
)

type RuleRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type Disposition struct {
	Value  string
	Clause string
}

type RuntimePolicy struct {
	ActionDispositions  map[string]Disposition
	ConfidenceThreshold string
	ApproverCount       int
}

type DecisionResult struct {
	Decision      Decision `json:"decision"`
	RuleFired     RuleRef  `json:"ruleFired"`
	Reason        string   `json:"reason"`
	AuditRef      string   `json:"auditRef"`
	Token         *string  `json:"token"`
	EscalationRef *string  `json:"escalationRef"`
}

func Decide(input DecisionInput, runtime RuntimePolicy) DecisionResult {
	if input.ActionClass == "" || input.PayloadDigest == "" || input.MandateRef == "" {
		return DecisionResult{
			Decision:  Deny,
			RuleFired: RuleRef{ID: "R-RUNTIME-VALIDATION", Version: 1},
			Reason:    "runtime validation failed",
		}
	}

	disposition, ok := runtime.ActionDispositions[input.ActionClass]
	if !ok {
		return DecisionResult{
			Decision:  Deny,
			RuleFired: RuleRef{ID: "R-ACTION-ABSENT", Version: 1},
			Reason:    "action class absent from mandate",
		}
	}

	if disposition.Value == "forbid" {
		return DecisionResult{
			Decision:  Deny,
			RuleFired: RuleRef{ID: disposition.Clause, Version: 1},
			Reason:    "action class forbidden by mandate clause " + disposition.Clause,
		}
	}

	if disposition.Value == "escalate" {
		return DecisionResult{
			Decision:  Escalate,
			RuleFired: RuleRef{ID: "R-MANDATE-ESCALATE", Version: 1},
			Reason:    "mandate reserves this action for human approval",
		}
	}

	if runtime.ConfidenceThreshold != "" && runtime.ApproverCount > 0 {
		cmp, err := CompareDecimalStrings(input.Confidence, runtime.ConfidenceThreshold)
		if err != nil || cmp < 0 {
			return DecisionResult{
				Decision:  Escalate,
				RuleFired: RuleRef{ID: "R-CONFIDENCE-LOW", Version: 1},
				Reason:    "confidence below mandate threshold",
			}
		}
	}

	if disposition.Value == "permit" {
		return DecisionResult{
			Decision:  Authorise,
			RuleFired: RuleRef{ID: "R-PERMIT", Version: 1},
			Reason:    "all validations passed and mandate permits action",
		}
	}

	return DecisionResult{
		Decision:  Deny,
		RuleFired: RuleRef{ID: "R-DISPOSITION-INVALID", Version: 1},
		Reason:    "invalid action disposition",
	}
}

func DefaultCVRuntimePolicy() RuntimePolicy {
	return RuntimePolicy{
		ActionDispositions: map[string]Disposition{
			"RECORD_ANNOTATE": {Value: "permit", Clause: "P1"},
			"RECORD_MODIFY":   {Value: "escalate", Clause: "E1"},
			"CANDIDATE_RANK":  {Value: "forbid", Clause: "D1"},
			"OUTBOUND_COMMS":  {Value: "forbid", Clause: "D2"},
		},
		ConfidenceThreshold: "0.70",
		ApproverCount:       1,
	}
}
