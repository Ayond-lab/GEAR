package exectoken

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
)

var ErrInvalidToken = errors.New("invalid execution token")

type Claims struct {
	ActionRef      string `json:"actionRef"`
	Connector      string `json:"connector"`
	Scope          string `json:"scope"`
	PayloadDigest  string `json:"payloadDigest"`
	MandateVersion int    `json:"mandateVersion"`
	Audience       string `json:"aud"`
	ExpiresAt      int64  `json:"exp"`
	JTI            string `json:"jti"`
}

type joseHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

func SignES256(key *ecdsa.PrivateKey, claims Claims) (string, error) {
	if key == nil {
		return "", errors.New("execution token private key is nil")
	}
	header, err := json.Marshal(joseHeader{Algorithm: "ES256", Type: "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := b64(header) + "." + b64(payload)
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

func VerifyES256(publicKey *ecdsa.PublicKey, token string) (Claims, error) {
	if publicKey == nil {
		return Claims{}, fmt.Errorf("%w: public key is nil", ErrInvalidToken)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("%w: token must contain three JWS segments", ErrInvalidToken)
	}

	headerBytes, err := unb64(parts[0])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: malformed header", ErrInvalidToken)
	}
	var header joseHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return Claims{}, fmt.Errorf("%w: malformed header json", ErrInvalidToken)
	}
	if header.Algorithm != "ES256" || header.Type != "JWT" {
		return Claims{}, fmt.Errorf("%w: unsupported header", ErrInvalidToken)
	}

	payloadBytes, err := unb64(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: malformed payload", ErrInvalidToken)
	}
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: malformed claims", ErrInvalidToken)
	}

	signature, err := unb64(parts[2])
	if err != nil || len(signature) != 64 {
		return Claims{}, fmt.Errorf("%w: malformed signature", ErrInvalidToken)
	}
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(publicKey, digest[:], r, s) {
		return Claims{}, fmt.Errorf("%w: signature verification failed", ErrInvalidToken)
	}
	return claims, nil
}

func DevelopmentPrivateKey() *ecdsa.PrivateKey {
	return DeterministicPrivateKey("gear-development-es256-execution-token-key")
}

func DeterministicPrivateKey(label string) *ecdsa.PrivateKey {
	curve := elliptic.P256()
	digest := sha256.Sum256([]byte(label))
	d := new(big.Int).SetBytes(digest[:])
	d.Mod(d, new(big.Int).Sub(curve.Params().N, big.NewInt(1)))
	d.Add(d, big.NewInt(1))
	x, y := curve.ScalarBaseMult(d.Bytes())
	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y},
		D:         d,
	}
}

func DevelopmentPublicKey() *ecdsa.PublicKey {
	return &DevelopmentPrivateKey().PublicKey
}

func LoadPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return PrivateKeyFromPEM(data)
}

func LoadPublicKey(path string) (*ecdsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return PublicKeyFromPEM(data)
}

func PrivateKeyFromPEM(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not ECDSA")
	}
	return key, nil
}

func PublicKeyFromPEM(data []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ECDSA")
	}
	return key, nil
}

func b64(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func unb64(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}
