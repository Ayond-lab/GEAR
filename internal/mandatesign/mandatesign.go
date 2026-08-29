package mandatesign

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	gearv1 "gear/api/v1"
	"gear/internal/exectoken"

	"github.com/gowebpki/jcs"
)

var ErrInvalidSignature = errors.New("invalid mandate signature")

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

func DevelopmentPrivateKey() *ecdsa.PrivateKey {
	return exectoken.DeterministicPrivateKey("gear-development-es256-mandate-signing-key")
}

func DevelopmentPublicKey() *ecdsa.PublicKey {
	return &DevelopmentPrivateKey().PublicKey
}

func Sign(spec gearv1.MandateSpec, key *ecdsa.PrivateKey) (string, error) {
	if key == nil {
		return "", errors.New("mandate signing key is nil")
	}
	payload, err := SigningPayload(spec)
	if err != nil {
		return "", err
	}
	headerBytes, err := json.Marshal(header{Algorithm: "ES256", Type: "gear-mandate+jws"})
	if err != nil {
		return "", err
	}
	signingInput := b64(headerBytes) + "." + b64(payload)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return signingInput + "." + b64(signature), nil
}

func Verify(spec gearv1.MandateSpec, publicKey *ecdsa.PublicKey) error {
	if spec.Signature == "" {
		return fmt.Errorf("%w: missing signature", ErrInvalidSignature)
	}
	if publicKey == nil {
		return fmt.Errorf("%w: public key is nil", ErrInvalidSignature)
	}
	parts := strings.Split(spec.Signature, ".")
	if len(parts) != 3 {
		return fmt.Errorf("%w: signature must contain three JWS segments", ErrInvalidSignature)
	}
	headerBytes, err := unb64(parts[0])
	if err != nil {
		return fmt.Errorf("%w: malformed header", ErrInvalidSignature)
	}
	var hdr header
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		return fmt.Errorf("%w: malformed header json", ErrInvalidSignature)
	}
	if hdr.Algorithm != "ES256" || hdr.Type != "gear-mandate+jws" {
		return fmt.Errorf("%w: unsupported header", ErrInvalidSignature)
	}
	payload, err := unb64(parts[1])
	if err != nil {
		return fmt.Errorf("%w: malformed payload", ErrInvalidSignature)
	}
	expectedPayload, err := SigningPayload(spec)
	if err != nil {
		return err
	}
	if !bytes.Equal(payload, expectedPayload) {
		return fmt.Errorf("%w: payload mismatch", ErrInvalidSignature)
	}
	signature, err := unb64(parts[2])
	if err != nil || len(signature) != 64 {
		return fmt.Errorf("%w: malformed signature", ErrInvalidSignature)
	}
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(publicKey, digest[:], r, s) {
		return fmt.Errorf("%w: verification failed", ErrInvalidSignature)
	}
	return nil
}

func SigningPayload(spec gearv1.MandateSpec) ([]byte, error) {
	spec.Signature = ""
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	return jcs.Transform(data)
}

func b64(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func unb64(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}
