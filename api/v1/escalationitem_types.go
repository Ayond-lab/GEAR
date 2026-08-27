package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// EscalationItem is the human decision record for a reserved or uncertain
// action.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=esc
type EscalationItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EscalationItemSpec   `json:"spec,omitempty"`
	Status EscalationItemStatus `json:"status,omitempty"`
}

type EscalationItemSpec struct {
	ActionRef    string      `json:"actionRef"`
	Reason       string      `json:"reason"`
	RuleFired    RuleRef     `json:"ruleFired"`
	EvidenceRefs []string    `json:"evidenceRefs"`
	ApproverSet  []string    `json:"approverSet"`
	CreatedAt    metav1.Time `json:"createdAt"`
}

type EscalationItemStatus struct {
	Decision  string      `json:"decision"`
	DecidedBy string      `json:"decidedBy"`
	DecidedAt metav1.Time `json:"decidedAt"`
	ReasonRef string      `json:"reasonRef"`
}

// EscalationItemList contains a list of EscalationItem resources.
//
// +kubebuilder:object:root=true
type EscalationItemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EscalationItem `json:"items"`
}
