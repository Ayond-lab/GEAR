package pepcore

import (
	"errors"
	"fmt"
	"time"
)

var ErrTokenRejected = errors.New("execution token rejected")

type Claims struct {
	ActionRef      string
	Connector      string
	Scope          string
	PayloadDigest  string
	MandateVersion int
	Audience       string
	ExpiresAt      time.Time
	JTI            string
}

type EffectRequest struct {
	ActionRef      string
	Connector      string
	Scope          string
	PayloadDigest  string
	MandateVersion int
}

type TokenVerifier struct {
	seen map[string]bool
	now  func() time.Time
}

func NewTokenVerifier() *TokenVerifier {
	return &TokenVerifier{
		seen: make(map[string]bool),
		now:  time.Now,
	}
}

func (v *TokenVerifier) Verify(claims Claims, request EffectRequest, allowedScopes map[string]bool) error {
	if v.now == nil {
		v.now = time.Now
	}
	if claims.Audience != "gear-pep" {
		return fmt.Errorf("%w: wrong audience", ErrTokenRejected)
	}
	if !claims.ExpiresAt.After(v.now()) {
		return fmt.Errorf("%w: expired", ErrTokenRejected)
	}
	if claims.JTI == "" || v.seen[claims.JTI] {
		return fmt.Errorf("%w: replayed jti", ErrTokenRejected)
	}
	if claims.ActionRef != request.ActionRef {
		return fmt.Errorf("%w: actionRef mismatch", ErrTokenRejected)
	}
	if claims.PayloadDigest != request.PayloadDigest {
		return fmt.Errorf("%w: payloadDigest mismatch", ErrTokenRejected)
	}
	if claims.MandateVersion != request.MandateVersion {
		return fmt.Errorf("%w: mandateVersion mismatch", ErrTokenRejected)
	}
	if claims.Connector != request.Connector || claims.Scope != request.Scope {
		return fmt.Errorf("%w: connector scope mismatch", ErrTokenRejected)
	}
	if !allowedScopes[claims.Connector+":"+claims.Scope] {
		return fmt.Errorf("%w: connector scope unsupported", ErrTokenRejected)
	}
	v.seen[claims.JTI] = true
	return nil
}

