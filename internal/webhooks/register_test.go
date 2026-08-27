package webhooks

import (
	"context"
	"errors"
	"net/http"
	"testing"

	gearv1 "gear/api/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestRegisterMandateWebhook(t *testing.T) {
	server := &recordingServer{}
	err := RegisterMandateWebhook(server, testScheme(t), NewMandateValidator(fakeClient(t, cvScreenAbility())))
	if err != nil {
		t.Fatalf("expected webhook registration to succeed, got %v", err)
	}

	if server.path != MandateValidationPath {
		t.Fatalf("expected path %q, got %q", MandateValidationPath, server.path)
	}
	if server.handler == nil {
		t.Fatal("expected handler to be registered")
	}
}

func TestRegisterMandateWebhookRejectsNilServer(t *testing.T) {
	err := RegisterMandateWebhook(nil, testScheme(t), NewMandateValidator(fakeClient(t)))
	if !errors.Is(err, ErrNilWebhookServer) {
		t.Fatalf("expected nil server error, got %v", err)
	}
}

func TestRegisterMandateWebhookRejectsNilScheme(t *testing.T) {
	err := RegisterMandateWebhook(&recordingServer{}, nil, NewMandateValidator(fakeClient(t)))
	if !errors.Is(err, ErrNilScheme) {
		t.Fatalf("expected nil scheme error, got %v", err)
	}
}

func TestMandateValidatorImplementsCustomValidator(t *testing.T) {
	var validator admission.CustomValidator = NewMandateValidator(fakeClient(t, cvScreenAbility()))
	warnings, err := validator.ValidateCreate(context.Background(), narrowedMandate())
	if err != nil {
		t.Fatalf("expected custom validator to accept mandate, got %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
}

func TestMandateValidatorRejectsUnexpectedObject(t *testing.T) {
	validator := NewMandateValidator(fakeClient(t, cvScreenAbility()))
	_, err := validator.ValidateCreate(context.Background(), &gearv1.Ability{})
	if !errors.Is(err, ErrUnexpectedObject) {
		t.Fatalf("expected unexpected object error, got %v", err)
	}
}

type recordingServer struct {
	path    string
	handler http.Handler
}

func (s *recordingServer) Register(path string, hook http.Handler) {
	s.path = path
	s.handler = hook
}

var _ registeringWebhookServer = (*recordingServer)(nil)
var _ runtime.Object = (*gearv1.Mandate)(nil)
