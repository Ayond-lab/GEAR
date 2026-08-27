package webhooks

import (
	"context"
	"errors"
	"strings"
	"testing"

	gearv1 "gear/api/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateMandateSubsumptionAllowsNarrowedMandate(t *testing.T) {
	err := ValidateMandateSubsumption(cvScreenAbilitySpec(), narrowedMandateSpec())
	if err != nil {
		t.Fatalf("expected narrowed mandate to pass, got %v", err)
	}
}

func TestValidateMandateSubsumptionRejectsActionOutsideManifest(t *testing.T) {
	mandate := narrowedMandateSpec()
	mandate.ActionGrants = append(mandate.ActionGrants, gearv1.ActionGrant{Class: "DELETE_RECORD", Disposition: "permit"})

	err := ValidateMandateSubsumption(cvScreenAbilitySpec(), mandate)

	if err == nil {
		t.Fatal("expected widened mandate rejection")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected Kubernetes invalid error, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "action class is outside manifest") {
		t.Fatalf("expected manifest violation in error, got %v", err)
	}
}

func TestMandateValidatorFetchesAbilityAndAllowsValidMandate(t *testing.T) {
	validator := NewMandateValidator(fakeClient(t, cvScreenAbility()))
	mandate := narrowedMandate()

	err := validator.ValidateCreate(context.Background(), mandate)

	if err != nil {
		t.Fatalf("expected mandate to be accepted, got %v", err)
	}
}

func TestMandateValidatorRejectsUnknownAbility(t *testing.T) {
	validator := NewMandateValidator(fakeClient(t))
	mandate := narrowedMandate()

	err := validator.ValidateCreate(context.Background(), mandate)

	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error for unknown ability, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "spec.abilityRef") {
		t.Fatalf("expected abilityRef field error, got %v", err)
	}
}

func TestMandateValidatorRejectsWidenedConnectorGrant(t *testing.T) {
	validator := NewMandateValidator(fakeClient(t, cvScreenAbility()))
	mandate := narrowedMandate()
	mandate.Spec.ConnectorGrants = append(mandate.Spec.ConnectorGrants, gearv1.ConnectorScope{Connector: "mail", Scope: "send"})

	err := validator.ValidateCreate(context.Background(), mandate)

	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error for widened connector grant, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "connector grant is outside manifest scopes") {
		t.Fatalf("expected connector violation in error, got %v", err)
	}
}

func TestMandateValidatorRejectsNilReader(t *testing.T) {
	err := NewMandateValidator(nil).ValidateCreate(context.Background(), narrowedMandate())
	if !errors.Is(err, ErrNilReader) {
		t.Fatalf("expected nil reader error, got %v", err)
	}
}

func TestMandateValidatorRejectsNilMandate(t *testing.T) {
	err := NewMandateValidator(fakeClient(t)).ValidateCreate(context.Background(), nil)
	if !errors.Is(err, ErrNilMandate) {
		t.Fatalf("expected nil mandate error, got %v", err)
	}
}

func fakeClient(t *testing.T, objects ...runtime.Object) client.Reader {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := gearv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
}

func cvScreenAbility() *gearv1.Ability {
	return &gearv1.Ability{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gearv1.GroupVersion.String(),
			Kind:       "Ability",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cv-screen",
			Namespace: "gear-lab",
		},
		Spec: cvScreenAbilitySpec(),
	}
}

func cvScreenAbilitySpec() gearv1.AbilitySpec {
	return gearv1.AbilitySpec{
		Version:       "0.3.0",
		Certification: "certified",
		DeclaredTriggers: []gearv1.TriggerDecl{
			{Type: "folder", ID: "applications-inbox"},
		},
		ConnectorScopes: []gearv1.ConnectorScope{
			{Connector: "applications-store", Scope: "read"},
			{Connector: "candidate-record", Scope: "write"},
		},
		ActionClasses: []string{"RECORD_ANNOTATE", "RECORD_MODIFY", "CANDIDATE_RANK", "OUTBOUND_COMMS"},
		Reversibility: map[string]string{
			"RECORD_ANNOTATE": "reversible",
			"RECORD_MODIFY":   "reversible",
			"CANDIDATE_RANK":  "reversible",
			"OUTBOUND_COMMS":  "irreversible",
		},
		DataClasses: []string{"personal", "protected-employment"},
		Ceilings:    gearv1.Ceilings{DailyActions: 500},
	}
}

func narrowedMandate() *gearv1.Mandate {
	return &gearv1.Mandate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gearv1.GroupVersion.String(),
			Kind:       "Mandate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mnd-2026-021",
			Namespace: "gear-lab",
		},
		Spec: narrowedMandateSpec(),
	}
}

func narrowedMandateSpec() gearv1.MandateSpec {
	return gearv1.MandateSpec{
		MandateID:      "MND-2026-021",
		Version:        2,
		AbilityRef:     "cv-screen",
		AbilityVersion: "0.3.0",
		Sources: []gearv1.Source{
			{Type: "folder", ID: "applications-inbox"},
		},
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
	}
}
