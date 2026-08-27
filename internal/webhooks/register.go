package webhooks

import (
	"errors"
	"net/http"

	gearv1 "gear/api/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	MandateValidationPath = "/validate-gear-eu-v1-mandate"
	PodMutationPath       = "/mutate-v1-pod"
)

var (
	ErrNilScheme        = errors.New("webhook registration requires a scheme")
	ErrNilWebhookServer = errors.New("webhook registration requires a webhook server")
)

type registeringWebhookServer interface {
	Register(path string, hook http.Handler)
}

func RegisterMandateWebhook(server registeringWebhookServer, scheme *runtime.Scheme, validator admission.CustomValidator) error {
	if server == nil {
		return ErrNilWebhookServer
	}
	if scheme == nil {
		return ErrNilScheme
	}
	if validator == nil {
		return ErrNilMandate
	}

	server.Register(
		MandateValidationPath,
		admission.WithCustomValidator(scheme, &gearv1.Mandate{}, validator),
	)
	return nil
}

func RegisterMandateWebhookWithManager(server webhook.Server, scheme *runtime.Scheme, reader client.Reader) error {
	return RegisterMandateWebhook(server, scheme, NewMandateValidator(reader))
}

func RegisterPodMutationWebhook(server registeringWebhookServer, scheme *runtime.Scheme, mutator admission.CustomDefaulter) error {
	if server == nil {
		return ErrNilWebhookServer
	}
	if scheme == nil {
		return ErrNilScheme
	}
	if mutator == nil {
		return ErrNilDefaulter
	}

	server.Register(
		PodMutationPath,
		admission.WithCustomDefaulter(scheme, &corev1.Pod{}, mutator),
	)
	return nil
}

func RegisterPodMutationWebhookWithManager(server webhook.Server, scheme *runtime.Scheme) error {
	return RegisterPodMutationWebhook(server, scheme, NewPodMutator())
}
