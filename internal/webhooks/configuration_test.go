package webhooks

import (
	"os"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestValidatingWebhookConfigurationSafetySettings(t *testing.T) {
	config := readValidatingWebhookConfiguration(t)
	if len(config.Webhooks) != 1 {
		t.Fatalf("expected one validating webhook, got %d", len(config.Webhooks))
	}

	webhook := config.Webhooks[0]
	if webhook.FailurePolicy == nil || *webhook.FailurePolicy != admissionregistrationv1.Fail {
		t.Fatalf("expected failurePolicy Fail, got %#v", webhook.FailurePolicy)
	}
	if webhook.SideEffects == nil || *webhook.SideEffects != admissionregistrationv1.SideEffectClassNone {
		t.Fatalf("expected sideEffects None, got %#v", webhook.SideEffects)
	}
	if !contains(webhook.AdmissionReviewVersions, "v1") {
		t.Fatalf("expected admissionReviewVersions to include v1, got %#v", webhook.AdmissionReviewVersions)
	}
	if webhook.ClientConfig.Service == nil {
		t.Fatal("expected service client config")
	}
	if webhook.ClientConfig.Service.Namespace != "gear-system" || webhook.ClientConfig.Service.Name != "gear-webhooks" {
		t.Fatalf("unexpected service target: %#v", webhook.ClientConfig.Service)
	}
	if webhook.ClientConfig.Service.Path == nil || *webhook.ClientConfig.Service.Path != MandateValidationPath {
		t.Fatalf("expected path %q, got %#v", MandateValidationPath, webhook.ClientConfig.Service.Path)
	}
}

func TestValidatingWebhookConfigurationRules(t *testing.T) {
	config := readValidatingWebhookConfiguration(t)
	webhook := config.Webhooks[0]
	if len(webhook.Rules) != 1 {
		t.Fatalf("expected one rule, got %d", len(webhook.Rules))
	}

	rule := webhook.Rules[0]
	if !contains(rule.APIGroups, "gear.eu") {
		t.Fatalf("expected gear.eu api group, got %#v", rule.APIGroups)
	}
	if !contains(rule.APIVersions, "v1") {
		t.Fatalf("expected v1 api version, got %#v", rule.APIVersions)
	}
	if !containsOperation(rule.Operations, admissionregistrationv1.Create) || !containsOperation(rule.Operations, admissionregistrationv1.Update) {
		t.Fatalf("expected CREATE and UPDATE operations, got %#v", rule.Operations)
	}
	if !contains(rule.Resources, "mandates") {
		t.Fatalf("expected mandates resource, got %#v", rule.Resources)
	}
	if rule.Scope == nil || *rule.Scope != admissionregistrationv1.NamespacedScope {
		t.Fatalf("expected namespaced scope, got %#v", rule.Scope)
	}
}

func TestValidatingWebhookConfigurationExcludesGearSystem(t *testing.T) {
	config := readValidatingWebhookConfiguration(t)
	selector := config.Webhooks[0].NamespaceSelector
	if selector == nil {
		t.Fatal("expected namespaceSelector")
	}

	for _, expression := range selector.MatchExpressions {
		if expression.Key == "kubernetes.io/metadata.name" &&
			expression.Operator == metav1.LabelSelectorOpNotIn &&
			contains(expression.Values, "gear-system") {
			return
		}
	}
	t.Fatalf("expected namespaceSelector to exclude gear-system, got %#v", selector.MatchExpressions)
}

func TestMutatingWebhookConfigurationSafetySettings(t *testing.T) {
	config := readMutatingWebhookConfiguration(t)
	if len(config.Webhooks) != 1 {
		t.Fatalf("expected one mutating webhook, got %d", len(config.Webhooks))
	}

	webhook := config.Webhooks[0]
	if webhook.FailurePolicy == nil || *webhook.FailurePolicy != admissionregistrationv1.Fail {
		t.Fatalf("expected failurePolicy Fail, got %#v", webhook.FailurePolicy)
	}
	if webhook.SideEffects == nil || *webhook.SideEffects != admissionregistrationv1.SideEffectClassNone {
		t.Fatalf("expected sideEffects None, got %#v", webhook.SideEffects)
	}
	if !contains(webhook.AdmissionReviewVersions, "v1") {
		t.Fatalf("expected admissionReviewVersions to include v1, got %#v", webhook.AdmissionReviewVersions)
	}
	if webhook.ClientConfig.Service == nil {
		t.Fatal("expected service client config")
	}
	if webhook.ClientConfig.Service.Namespace != "gear-system" || webhook.ClientConfig.Service.Name != "gear-webhooks" {
		t.Fatalf("unexpected service target: %#v", webhook.ClientConfig.Service)
	}
	if webhook.ClientConfig.Service.Path == nil || *webhook.ClientConfig.Service.Path != PodMutationPath {
		t.Fatalf("expected path %q, got %#v", PodMutationPath, webhook.ClientConfig.Service.Path)
	}
}

func TestMutatingWebhookConfigurationRules(t *testing.T) {
	config := readMutatingWebhookConfiguration(t)
	webhook := config.Webhooks[0]
	if len(webhook.Rules) != 1 {
		t.Fatalf("expected one rule, got %d", len(webhook.Rules))
	}

	rule := webhook.Rules[0]
	if !contains(rule.APIGroups, "") {
		t.Fatalf("expected core api group, got %#v", rule.APIGroups)
	}
	if !contains(rule.APIVersions, "v1") {
		t.Fatalf("expected v1 api version, got %#v", rule.APIVersions)
	}
	if !containsOperation(rule.Operations, admissionregistrationv1.Create) {
		t.Fatalf("expected CREATE operation, got %#v", rule.Operations)
	}
	if !contains(rule.Resources, "pods") {
		t.Fatalf("expected pods resource, got %#v", rule.Resources)
	}
	if rule.Scope == nil || *rule.Scope != admissionregistrationv1.NamespacedScope {
		t.Fatalf("expected namespaced scope, got %#v", rule.Scope)
	}
}

func TestMutatingWebhookConfigurationSelectors(t *testing.T) {
	config := readMutatingWebhookConfiguration(t)
	webhook := config.Webhooks[0]
	if !selectorExcludesGearSystem(webhook.NamespaceSelector) {
		t.Fatalf("expected namespaceSelector to exclude gear-system, got %#v", webhook.NamespaceSelector)
	}
	if webhook.ObjectSelector == nil {
		t.Fatal("expected objectSelector for ability pods")
	}
	for _, expression := range webhook.ObjectSelector.MatchExpressions {
		if expression.Key == AbilityLabel && expression.Operator == metav1.LabelSelectorOpExists {
			return
		}
	}
	t.Fatalf("expected objectSelector to match pods labelled %s, got %#v", AbilityLabel, webhook.ObjectSelector.MatchExpressions)
}

func readValidatingWebhookConfiguration(t *testing.T) admissionregistrationv1.ValidatingWebhookConfiguration {
	t.Helper()
	data, err := os.ReadFile("../../deploy/base/webhook-validatingconfiguration.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var config admissionregistrationv1.ValidatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func readMutatingWebhookConfiguration(t *testing.T) admissionregistrationv1.MutatingWebhookConfiguration {
	t.Helper()
	data, err := os.ReadFile("../../deploy/base/webhook-mutatingconfiguration.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var config admissionregistrationv1.MutatingWebhookConfiguration
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func selectorExcludesGearSystem(selector *metav1.LabelSelector) bool {
	if selector == nil {
		return false
	}
	for _, expression := range selector.MatchExpressions {
		if expression.Key == "kubernetes.io/metadata.name" &&
			expression.Operator == metav1.LabelSelectorOpNotIn &&
			contains(expression.Values, "gear-system") {
			return true
		}
	}
	return false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsOperation(values []admissionregistrationv1.OperationType, target admissionregistrationv1.OperationType) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
