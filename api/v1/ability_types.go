package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Ability is the publisher-declared capability boundary for one ability
// version.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ability
type Ability struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AbilitySpec   `json:"spec,omitempty"`
	Status AbilityStatus `json:"status,omitempty"`
}

type AbilitySpec struct {
	Publisher        Publisher         `json:"publisher"`
	Version          string            `json:"version"`
	ManifestDigest   string            `json:"manifestDigest"`
	Certification    string            `json:"certificationStatus"`
	DeclaredTriggers []TriggerDecl     `json:"declaredTriggers"`
	ConnectorScopes  []ConnectorScope  `json:"connectorScopes"`
	ActionClasses    []string          `json:"actionClasses"`
	Reversibility    map[string]string `json:"reversibilityClasses"`
	DataClasses      []string          `json:"dataClasses"`
	Ceilings         Ceilings          `json:"ceilings"`
}

type AbilityStatus struct {
	ObservedVersion string `json:"observedVersion,omitempty"`
	Valid           bool   `json:"valid,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// AbilityList contains a list of Ability resources.
//
// +kubebuilder:object:root=true
type AbilityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ability `json:"items"`
}
