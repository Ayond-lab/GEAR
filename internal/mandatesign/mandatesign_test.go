package mandatesign

import (
	"errors"
	"strings"
	"testing"
	"time"

	gearv1 "gear/api/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSignAndVerifyMandateSpec(t *testing.T) {
	spec := fixtureMandateSpec()
	signature, err := Sign(spec, DevelopmentPrivateKey())
	if err != nil {
		t.Fatal(err)
	}
	spec.Signature = signature

	if err := Verify(spec, DevelopmentPublicKey()); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsMandateTampering(t *testing.T) {
	spec := fixtureMandateSpec()
	signature, err := Sign(spec, DevelopmentPrivateKey())
	if err != nil {
		t.Fatal(err)
	}
	spec.Signature = signature
	spec.ActionGrants[0].Disposition = "forbid"

	err = Verify(spec, DevelopmentPublicKey())
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
}

func TestSigningPayloadExcludesSignature(t *testing.T) {
	spec := fixtureMandateSpec()
	spec.Signature = "placeholder"
	payload, err := SigningPayload(spec)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "placeholder") || strings.Contains(string(payload), "signature") {
		t.Fatalf("signature must not be part of signed payload: %s", payload)
	}
}

func fixtureMandateSpec() gearv1.MandateSpec {
	return gearv1.MandateSpec{
		MandateID:        "MND-2026-021",
		Version:          2,
		AbilityRef:       "cv-screen",
		AbilityVersion:   "0.3.0",
		PurposeStatement: "Identify candidates who will require work authorisation, for planning.",
		LegalBasis:       "Right-to-work verification",
		Sources:          []gearv1.Source{{Type: "folder", ID: "applications-inbox"}},
		ConnectorGrants: []gearv1.ConnectorScope{
			{Connector: "applications-store", Scope: "read"},
			{Connector: "candidate-record", Scope: "write"},
		},
		ActionGrants: []gearv1.ActionGrant{
			{Class: "RECORD_ANNOTATE", Disposition: "permit"},
			{Class: "RECORD_MODIFY", Disposition: "escalate"},
			{Class: "CANDIDATE_RANK", Disposition: "forbid"},
			{Class: "OUTBOUND_COMMS", Disposition: "forbid"},
		},
		Caps:       gearv1.Caps{DailyActions: 50},
		Thresholds: map[string]string{"extractionConfidence": "0.70"},
		Approvers:  []gearv1.Approver{{ID: "hiring-manager-1", Name: "Hiring Manager"}},
		Egress:     []gearv1.EgressRule{},
		ExpiresAt:  metav1.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC),
		CredentialRef: corev1.SecretReference{
			Name:      "mnd-2026-021-credential",
			Namespace: "gear-lab",
		},
	}
}
