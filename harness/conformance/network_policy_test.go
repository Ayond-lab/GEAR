package conformance

import (
	"os"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestAbilityEgressNetworkPolicyBaseline(t *testing.T) {
	data, err := os.ReadFile("../../deploy/network/ability-egress-baseline.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var policy networkingv1.NetworkPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}

	if policy.Namespace != "gear-lab" {
		t.Fatalf("expected smoke policy namespace gear-lab, got %q", policy.Namespace)
	}
	if !hasSelectorExpression(policy.Spec.PodSelector.MatchExpressions, "gear.eu/ability", metav1.LabelSelectorOpExists, nil) {
		t.Fatalf("expected policy to select ability pods, got %#v", policy.Spec.PodSelector.MatchExpressions)
	}
	if !hasPolicyType(policy.Spec.PolicyTypes, networkingv1.PolicyTypeEgress) {
		t.Fatalf("expected egress policy type, got %#v", policy.Spec.PolicyTypes)
	}
	if len(policy.Spec.Egress) != 2 {
		t.Fatalf("expected DNS and GEAR service egress rules, got %d", len(policy.Spec.Egress))
	}
	if !allowsDNS(policy.Spec.Egress[0]) {
		t.Fatalf("expected first egress rule to allow DNS, got %#v", policy.Spec.Egress[0])
	}
	if !allowsGearServices(policy.Spec.Egress[1]) {
		t.Fatalf("expected second egress rule to allow only trusted GEAR services, got %#v", policy.Spec.Egress[1])
	}
}

func hasSelectorExpression(expressions []metav1.LabelSelectorRequirement, key string, operator metav1.LabelSelectorOperator, values []string) bool {
	for _, expression := range expressions {
		if expression.Key != key || expression.Operator != operator {
			continue
		}
		if len(values) == 0 {
			return len(expression.Values) == 0
		}
		if stringSetEqual(expression.Values, values) {
			return true
		}
	}
	return false
}

func hasPolicyType(values []networkingv1.PolicyType, target networkingv1.PolicyType) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func allowsDNS(rule networkingv1.NetworkPolicyEgressRule) bool {
	if len(rule.To) != 1 {
		return false
	}
	peer := rule.To[0]
	if peer.NamespaceSelector == nil || peer.PodSelector == nil {
		return false
	}
	if !labelMatches(peer.NamespaceSelector.MatchLabels, "kubernetes.io/metadata.name", "kube-system") {
		return false
	}
	if !labelMatches(peer.PodSelector.MatchLabels, "k8s-app", "kube-dns") {
		return false
	}
	return hasPort(rule.Ports, "UDP", 53) && hasPort(rule.Ports, "TCP", 53)
}

func allowsGearServices(rule networkingv1.NetworkPolicyEgressRule) bool {
	if len(rule.To) != 1 {
		return false
	}
	peer := rule.To[0]
	if peer.NamespaceSelector == nil || peer.PodSelector == nil {
		return false
	}
	if !labelMatches(peer.NamespaceSelector.MatchLabels, "kubernetes.io/metadata.name", "gear-system") {
		return false
	}
	return hasSelectorExpression(
		peer.PodSelector.MatchExpressions,
		"app.kubernetes.io/name",
		metav1.LabelSelectorOpIn,
		[]string{"gear-policy", "gear-inference", "gear-fixture-store"},
	) && hasPort(rule.Ports, "TCP", 8080) && hasPort(rule.Ports, "TCP", 443)
}

func labelMatches(labels map[string]string, key string, value string) bool {
	return labels[key] == value
}

func hasPort(ports []networkingv1.NetworkPolicyPort, protocol string, port int32) bool {
	for _, value := range ports {
		if value.Protocol == nil || string(*value.Protocol) != protocol {
			continue
		}
		if value.Port == nil || value.Port.IntVal != port {
			continue
		}
		return true
	}
	return false
}

func stringSetEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	values := make(map[string]bool, len(left))
	for _, value := range left {
		values[value] = true
	}
	for _, value := range right {
		if !values[value] {
			return false
		}
	}
	return true
}
