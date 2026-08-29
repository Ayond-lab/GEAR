package exectoken

import (
	"errors"
	"testing"
)

func TestSignVerifyES256(t *testing.T) {
	claims := Claims{
		ActionRef:      "ga-001",
		Connector:      "candidate-record",
		Scope:          "write",
		PayloadDigest:  "sha256:payload",
		MandateVersion: 2,
		Audience:       "gear-pep",
		ExpiresAt:      1790000000,
		JTI:            "jti-001",
	}

	token, err := SignES256(DevelopmentPrivateKey(), claims)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyES256(DevelopmentPublicKey(), token)
	if err != nil {
		t.Fatal(err)
	}
	if verified != claims {
		t.Fatalf("claims mismatch: %#v", verified)
	}
}

func TestVerifyRejectsWrongSignature(t *testing.T) {
	claims := Claims{ActionRef: "ga-001", Audience: "gear-pep", ExpiresAt: 1790000000, JTI: "jti-001"}
	token, err := SignES256(DevelopmentPrivateKey(), claims)
	if err != nil {
		t.Fatal(err)
	}
	tampered := token[:len(token)-2] + "aa"

	_, err = VerifyES256(DevelopmentPublicKey(), tampered)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid-token error, got %v", err)
	}
}

func TestDevelopmentPublicKeyIsStable(t *testing.T) {
	first := DevelopmentPublicKey()
	second := DevelopmentPublicKey()
	if first.X.Cmp(second.X) != 0 || first.Y.Cmp(second.Y) != 0 {
		t.Fatal("expected deterministic development public key")
	}
}
