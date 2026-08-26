package v1

// Mandate is the principal-defined permission boundary for one ability version.
type Mandate struct {
	APIVersion string        `json:"apiVersion,omitempty"`
	Kind       string        `json:"kind,omitempty"`
	Metadata   ObjectMeta    `json:"metadata"`
	Spec       MandateSpec   `json:"spec"`
	Status     MandateStatus `json:"status,omitempty"`
}

type MandateSpec struct {
	MandateID        string           `json:"mandateId"`
	Version          int              `json:"version"`
	AbilityRef       string           `json:"abilityRef"`
	AbilityVersion   string           `json:"abilityVersion"`
	PurposeStatement string           `json:"purposeStatement"`
	LegalBasis       string           `json:"legalBasis,omitempty"`
	Sources          []Source         `json:"sources"`
	ConnectorGrants  []ConnectorScope `json:"connectorGrants"`
	ActionGrants     []ActionGrant    `json:"actionGrants"`
	Caps             Caps             `json:"caps"`
	Thresholds       map[string]string `json:"thresholds"`
	Approvers        []Approver       `json:"approvers"`
	Egress           []EgressRule     `json:"egress"`
	Placement        *Placement       `json:"placement,omitempty"`
	ExpiresAt        Time             `json:"expiresAt"`
	CredentialRef    SecretReference  `json:"credentialRef"`
	Signature        string           `json:"signature,omitempty"`
}

type MandateStatus struct {
	Active bool   `json:"active,omitempty"`
	Reason string `json:"reason,omitempty"`
}

