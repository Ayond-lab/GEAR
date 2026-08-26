package pepcore

import (
	"errors"
	"testing"
	"time"
)

func TestVerifyAcceptsSingleUseToken(t *testing.T) {
	verifier, claims, request := fixtureVerifier()

	if err := verifier.Verify(claims, request, allowedScopes()); err != nil {
		t.Fatalf("expected token to verify, got %v", err)
	}
}

func TestVerifyRejectsReplay(t *testing.T) {
	verifier, claims, request := fixtureVerifier()
	if err := verifier.Verify(claims, request, allowedScopes()); err != nil {
		t.Fatal(err)
	}
	err := verifier.Verify(claims, request, allowedScopes())

	if !errors.Is(err, ErrTokenRejected) {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func TestVerifyRejectsPayloadDigestMismatch(t *testing.T) {
	verifier, claims, request := fixtureVerifier()
	request.PayloadDigest = "sha256:other"

	err := verifier.Verify(claims, request, allowedScopes())

	if !errors.Is(err, ErrTokenRejected) {
		t.Fatalf("expected token rejection, got %v", err)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	verifier, claims, request := fixtureVerifier()
	claims.ExpiresAt = time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)

	err := verifier.Verify(claims, request, allowedScopes())

	if !errors.Is(err, ErrTokenRejected) {
		t.Fatalf("expected expiry rejection, got %v", err)
	}
}

func fixtureVerifier() (*TokenVerifier, Claims, EffectRequest) {
	now := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	verifier := NewTokenVerifier()
	verifier.now = func() time.Time { return now }
	claims := Claims{
		ActionRef:      "ga-001",
		Connector:      "candidate-record",
		Scope:          "write",
		PayloadDigest:  "sha256:payload",
		MandateVersion: 2,
		Audience:       "gear-pep",
		ExpiresAt:      now.Add(30 * time.Second),
		JTI:            "token-001",
	}
	request := EffectRequest{
		ActionRef:      "ga-001",
		Connector:      "candidate-record",
		Scope:          "write",
		PayloadDigest:  "sha256:payload",
		MandateVersion: 2,
	}
	return verifier, claims, request
}

func allowedScopes() map[string]bool {
	return map[string]bool{"candidate-record:write": true}
}

