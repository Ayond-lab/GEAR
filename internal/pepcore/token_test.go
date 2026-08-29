package pepcore

import (
	"errors"
	"testing"
	"time"

	"gear/internal/exectoken"
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

func TestVerifyJWSAcceptsSignedSingleUseToken(t *testing.T) {
	verifier, claims, request := fixtureVerifier()
	token := signedVerifierToken(t, claims)

	if err := verifier.VerifyJWS(token, request, allowedScopes()); err != nil {
		t.Fatalf("expected JWS to verify, got %v", err)
	}
	if err := verifier.VerifyJWS(token, request, allowedScopes()); !errors.Is(err, ErrTokenRejected) {
		t.Fatalf("expected JWS replay rejection, got %v", err)
	}
}

func TestVerifyJWSRejectsWrongAudience(t *testing.T) {
	verifier, claims, request := fixtureVerifier()
	claims.Audience = "other"
	token := signedVerifierToken(t, claims)

	err := verifier.VerifyJWS(token, request, allowedScopes())
	if !errors.Is(err, ErrTokenRejected) {
		t.Fatalf("expected wrong audience rejection, got %v", err)
	}
}

func TestVerifyJWSRejectsUnsupportedScope(t *testing.T) {
	verifier, claims, request := fixtureVerifier()
	claims.Scope = "admin"
	request.Scope = "admin"
	token := signedVerifierToken(t, claims)

	err := verifier.VerifyJWS(token, request, allowedScopes())
	if !errors.Is(err, ErrTokenRejected) {
		t.Fatalf("expected unsupported scope rejection, got %v", err)
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

func signedVerifierToken(t *testing.T, claims Claims) string {
	t.Helper()
	token, err := exectoken.SignES256(exectoken.DevelopmentPrivateKey(), exectoken.Claims{
		ActionRef:      claims.ActionRef,
		Connector:      claims.Connector,
		Scope:          claims.Scope,
		PayloadDigest:  claims.PayloadDigest,
		MandateVersion: claims.MandateVersion,
		Audience:       claims.Audience,
		ExpiresAt:      claims.ExpiresAt.Unix(),
		JTI:            claims.JTI,
	})
	if err != nil {
		t.Fatal(err)
	}
	return token
}
