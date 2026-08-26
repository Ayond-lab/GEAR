package v1

// EscalationItem is the human decision record for a reserved or uncertain
// action.
type EscalationItem struct {
	APIVersion string               `json:"apiVersion,omitempty"`
	Kind       string               `json:"kind,omitempty"`
	Metadata   ObjectMeta           `json:"metadata"`
	Spec       EscalationItemSpec   `json:"spec"`
	Status     EscalationItemStatus `json:"status,omitempty"`
}

type EscalationItemSpec struct {
	ActionRef    string   `json:"actionRef"`
	Reason       string   `json:"reason"`
	RuleFired    RuleRef  `json:"ruleFired"`
	EvidenceRefs []string `json:"evidenceRefs"`
	ApproverSet  []string `json:"approverSet"`
	CreatedAt    Time     `json:"createdAt"`
}

type EscalationItemStatus struct {
	Decision  string `json:"decision"`
	DecidedBy string `json:"decidedBy"`
	DecidedAt Time   `json:"decidedAt"`
	ReasonRef string `json:"reasonRef"`
}

