package v1

type ObjectMeta struct {
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type Time string

type SecretReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type Publisher struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type TriggerDecl struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type TriggerRef struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	EventID string `json:"eventId"`
}

type ConnectorScope struct {
	Connector string `json:"connector"`
	Scope     string `json:"scope"`
}

type Source struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type Ceilings struct {
	DailyActions int `json:"dailyActions,omitempty"`
}

type Caps struct {
	DailyActions int `json:"dailyActions,omitempty"`
}

type ActionGrant struct {
	Class       string `json:"class"`
	Disposition string `json:"disposition"`
}

type Approver struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type EgressRule struct {
	Host string `json:"host"`
	Port int    `json:"port,omitempty"`
}

type Placement struct {
	Region     string            `json:"region,omitempty"`
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`
}

type RuleRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

