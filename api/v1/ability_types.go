package v1

// Ability is the publisher-declared capability boundary for one ability
// version. Kubernetes status is represented here as a normal field until
// kubebuilder markers are added in Milestone 1.
type Ability struct {
	APIVersion string        `json:"apiVersion,omitempty"`
	Kind       string        `json:"kind,omitempty"`
	Metadata   ObjectMeta    `json:"metadata"`
	Spec       AbilitySpec   `json:"spec"`
	Status     AbilityStatus `json:"status,omitempty"`
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

