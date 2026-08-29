package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	gearv1 "gear/api/v1"
	"gear/internal/mandatesign"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
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

	err := validator.ValidateCreateMandate(context.Background(), mandate)

	if err != nil {
		t.Fatalf("expected mandate to be accepted, got %v", err)
	}
}

func TestMandateValidatorRejectsUnknownAbility(t *testing.T) {
	validator := NewMandateValidator(fakeClient(t))
	mandate := narrowedMandate()

	err := validator.ValidateCreateMandate(context.Background(), mandate)

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

	err := validator.ValidateCreateMandate(context.Background(), mandate)

	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error for widened connector grant, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "connector grant is outside manifest scopes") {
		t.Fatalf("expected connector violation in error, got %v", err)
	}
}

func TestMandateValidatorRejectsUnsignedMandate(t *testing.T) {
	validator := NewMandateValidator(fakeClient(t, cvScreenAbility()))
	mandate := narrowedMandate()
	mandate.Spec.Signature = ""

	err := validator.ValidateCreateMandate(context.Background(), mandate)

	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error for unsigned mandate, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "missing signature") {
		t.Fatalf("expected missing signature error, got %v", err)
	}
}

func TestMandateValidatorRejectsTamperedSignature(t *testing.T) {
	validator := NewMandateValidator(fakeClient(t, cvScreenAbility()))
	mandate := narrowedMandate()
	mandate.Spec.Caps.DailyActions = 51

	err := validator.ValidateCreateMandate(context.Background(), mandate)

	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error for tampered mandate, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "payload mismatch") {
		t.Fatalf("expected signature payload mismatch, got %v", err)
	}
}

func TestMandateValidatorRejectsLegalityRefusedCandidateRankPermit(t *testing.T) {
	validator := NewMandateValidator(fakeClient(t, cvScreenAbility()))
	mandate := narrowedMandate()
	mandate.Spec.PurposeStatement = "Check the CVs, select the candidates who are not citizens of the EEA."
	mandate.Spec.ActionGrants = []gearv1.ActionGrant{
		{Class: "RECORD_ANNOTATE", Disposition: "permit"},
		{Class: "CANDIDATE_RANK", Disposition: "permit"},
	}
	mandate.Spec.Signature = mustSignMandateSpec(mandate.Spec)

	err := validator.ValidateCreateMandate(context.Background(), mandate)

	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error for legality-refused mandate, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "CANDIDATE_RANK was refused by legality gate") {
		t.Fatalf("expected legality-gate action rejection, got %v", err)
	}
	if strings.Contains(err.Error(), "Check the CVs") || strings.Contains(err.Error(), "citizens of the EEA") {
		t.Fatalf("admission error should not echo raw unlawful purpose, got %v", err)
	}
}

func TestMandateValidatorRejectsNilReader(t *testing.T) {
	err := NewMandateValidator(nil).ValidateCreateMandate(context.Background(), narrowedMandate())
	if !errors.Is(err, ErrNilReader) {
		t.Fatalf("expected nil reader error, got %v", err)
	}
}

func TestMandateValidatorRejectsNilMandate(t *testing.T) {
	err := NewMandateValidator(fakeClient(t)).ValidateCreateMandate(context.Background(), nil)
	if !errors.Is(err, ErrNilMandate) {
		t.Fatalf("expected nil mandate error, got %v", err)
	}
}

func TestMandateAdmissionHandlerAllowsNarrowedMandate(t *testing.T) {
	scheme := testScheme(t)
	handler := admission.WithCustomValidator(
		scheme,
		&gearv1.Mandate{},
		NewMandateValidator(fakeClientWithScheme(t, scheme, cvScreenAbility())),
	)

	response := handler.Handle(context.Background(), admissionRequest(t, narrowedMandate()))

	if !response.Allowed {
		t.Fatalf("expected admission to allow narrowed mandate, got %#v", response.Result)
	}
}

func TestMandateAdmissionHandlerRejectsWidenedMandate(t *testing.T) {
	scheme := testScheme(t)
	mandate := narrowedMandate()
	mandate.Spec.ActionGrants = append(mandate.Spec.ActionGrants, gearv1.ActionGrant{Class: "DELETE_RECORD", Disposition: "permit"})
	handler := admission.WithCustomValidator(
		scheme,
		&gearv1.Mandate{},
		NewMandateValidator(fakeClientWithScheme(t, scheme, cvScreenAbility())),
	)

	response := handler.Handle(context.Background(), admissionRequest(t, mandate))

	if response.Allowed {
		t.Fatal("expected admission to reject widened mandate")
	}
	if response.Result.Code == http.StatusOK {
		t.Fatalf("expected non-OK admission status, got %#v", response.Result)
	}
	if !strings.Contains(response.Result.Message, "action class is outside manifest") {
		t.Fatalf("expected subsumption violation in admission response, got %#v", response.Result)
	}
}

func fakeClient(t *testing.T, objects ...runtime.Object) client.Reader {
	t.Helper()
	return fakeClientWithScheme(t, testScheme(t), objects...)
}

func fakeClientWithScheme(t *testing.T, scheme *runtime.Scheme, objects ...runtime.Object) client.Reader {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := gearv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func admissionRequest(t *testing.T, mandate *gearv1.Mandate) admission.Request {
	t.Helper()
	data, err := json.Marshal(mandate)
	if err != nil {
		t.Fatal(err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-request",
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: data,
			},
		},
	}
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
	spec := gearv1.MandateSpec{
		MandateID:        "MND-2026-021",
		Version:          2,
		AbilityRef:       "cv-screen",
		AbilityVersion:   "0.3.0",
		PurposeStatement: "Identify candidates who will require work authorisation, for planning.",
		LegalBasis:       "Right-to-work verification",
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
		Approvers: []gearv1.Approver{
			{ID: "hiring-manager-1", Name: "Hiring Manager"},
		},
		Egress:    []gearv1.EgressRule{},
		ExpiresAt: metav1.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC),
		CredentialRef: corev1.SecretReference{
			Name:      "mnd-2026-021-credential",
			Namespace: "gear-lab",
		},
	}
	spec.Signature = mustSignMandateSpec(spec)
	return spec
}

func mustSignMandateSpec(spec gearv1.MandateSpec) string {
	signature, err := mandatesign.Sign(spec, mandatesign.DevelopmentPrivateKey())
	if err != nil {
		panic(err)
	}
	return signature
}
