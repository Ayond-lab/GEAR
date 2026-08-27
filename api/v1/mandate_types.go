package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mandate is the principal-defined permission boundary for one ability version.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=mandate
type Mandate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MandateSpec   `json:"spec,omitempty"`
	Status MandateStatus `json:"status,omitempty"`
}

type MandateSpec struct {
	MandateID        string                 `json:"mandateId"`
	Version          int                    `json:"version"`
	AbilityRef       string                 `json:"abilityRef"`
	AbilityVersion   string                 `json:"abilityVersion"`
	PurposeStatement string                 `json:"purposeStatement"`
	LegalBasis       string                 `json:"legalBasis,omitempty"`
	Sources          []Source               `json:"sources"`
	ConnectorGrants  []ConnectorScope       `json:"connectorGrants"`
	ActionGrants     []ActionGrant          `json:"actionGrants"`
	Caps             Caps                   `json:"caps"`
	Thresholds       map[string]string      `json:"thresholds"`
	Approvers        []Approver             `json:"approvers"`
	Egress           []EgressRule           `json:"egress"`
	Placement        *Placement             `json:"placement,omitempty"`
	ExpiresAt        metav1.Time            `json:"expiresAt"`
	CredentialRef    corev1.SecretReference `json:"credentialRef"`
	Signature        string                 `json:"signature,omitempty"`
}

type MandateStatus struct {
	Active bool   `json:"active,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// MandateList contains a list of Mandate resources.
//
// +kubebuilder:object:root=true
type MandateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mandate `json:"items"`
}
