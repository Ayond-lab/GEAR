package pepcore

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	"gear/internal/exectoken"
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
	seen      map[string]bool
	now       func() time.Time
	publicKey *ecdsa.PublicKey
}

func NewTokenVerifier() *TokenVerifier {
	return NewTokenVerifierWithKey(exectoken.DevelopmentPublicKey())
}

func NewTokenVerifierWithKey(publicKey *ecdsa.PublicKey) *TokenVerifier {
	return &TokenVerifier{
		seen:      make(map[string]bool),
		now:       time.Now,
		publicKey: publicKey,
	}
}

func (v *TokenVerifier) VerifyJWS(token string, request EffectRequest, allowedScopes map[string]bool) error {
	if v == nil {
		return fmt.Errorf("%w: verifier unavailable", ErrTokenRejected)
	}
	claims, err := exectoken.VerifyES256(v.publicKey, token)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTokenRejected, err)
	}
	return v.Verify(Claims{
		ActionRef:      claims.ActionRef,
		Connector:      claims.Connector,
		Scope:          claims.Scope,
		PayloadDigest:  claims.PayloadDigest,
		MandateVersion: claims.MandateVersion,
		Audience:       claims.Audience,
		ExpiresAt:      time.Unix(claims.ExpiresAt, 0),
		JTI:            claims.JTI,
	}, request, allowedScopes)
}

func (v *TokenVerifier) Verify(claims Claims, request EffectRequest, allowedScopes map[string]bool) error {
	if v == nil {
		return fmt.Errorf("%w: verifier unavailable", ErrTokenRejected)
	}
	if v.now == nil {
		v.now = time.Now
	}
	if v.seen == nil {
		v.seen = map[string]bool{}
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
