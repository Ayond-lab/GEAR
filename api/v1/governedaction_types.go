package v1

// GovernedAction is a runtime request for an action under a specific mandate.
type GovernedAction struct {
	APIVersion string               `json:"apiVersion,omitempty"`
	Kind       string               `json:"kind,omitempty"`
	Metadata   ObjectMeta           `json:"metadata"`
	Spec       GovernedActionSpec   `json:"spec"`
	Status     GovernedActionStatus `json:"status,omitempty"`
}

type GovernedActionSpec struct {
	ActionClass     string     `json:"actionClass"`
	PayloadDigest   string     `json:"payloadDigest"`
	IdempotencyKey  string     `json:"idempotencyKey"`
	AbilityRef      string     `json:"abilityRef"`
	AbilityVersion  string     `json:"abilityVersion"`
	MandateRef      string     `json:"mandateRef"`
	MandateVersion  int        `json:"mandateVersion"`
	SubjectRef      string     `json:"subjectRef"`
	DataClasses     []string   `json:"dataClasses"`
	Confidence      string     `json:"confidence"`
	TriggerRef      TriggerRef `json:"triggerRef"`
}

type GovernedActionStatus struct {
	Decision       string  `json:"decision"`
	RuleFired      RuleRef `json:"ruleFired"`
	ExecutionState string  `json:"executionState"`
	AuditRef       string  `json:"auditRef"`
	EffectRef      string  `json:"effectRef,omitempty"`
}

