package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// GovernedAction is a runtime request for an action under a specific mandate.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ga
type GovernedAction struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GovernedActionSpec   `json:"spec,omitempty"`
	Status GovernedActionStatus `json:"status,omitempty"`
}

type GovernedActionSpec struct {
	ActionClass    string     `json:"actionClass"`
	PayloadDigest  string     `json:"payloadDigest"`
	IdempotencyKey string     `json:"idempotencyKey"`
	AbilityRef     string     `json:"abilityRef"`
	AbilityVersion string     `json:"abilityVersion"`
	MandateRef     string     `json:"mandateRef"`
	MandateVersion int        `json:"mandateVersion"`
	SubjectRef     string     `json:"subjectRef"`
	DataClasses    []string   `json:"dataClasses"`
	Confidence     string     `json:"confidence"`
	TriggerRef     TriggerRef `json:"triggerRef"`
}

type GovernedActionStatus struct {
	Decision       string  `json:"decision"`
	RuleFired      RuleRef `json:"ruleFired"`
	ExecutionState string  `json:"executionState"`
	AuditRef       string  `json:"auditRef"`
	EffectRef      string  `json:"effectRef,omitempty"`
}

// GovernedActionList contains a list of GovernedAction resources.
//
// +kubebuilder:object:root=true
type GovernedActionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GovernedAction `json:"items"`
}
