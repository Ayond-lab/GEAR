package seams

import "context"

type Claims struct {
	Subject string
	Issuer  string
	Scopes  []string
}

type Status string

const (
	StatusValid   Status = "valid"
	StatusRevoked Status = "revoked"
	StatusUnknown Status = "unknown"
)

type CredentialVerifier interface {
	Verify(ctx context.Context, ref string) (Claims, error)
	RevocationStatus(ctx context.Context, ref string) (Status, error)
}

type NodeAttributeProvider interface {
	Attributes(ctx context.Context, node string) (map[string]string, error)
}

