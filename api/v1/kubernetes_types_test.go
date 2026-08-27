package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToSchemeRegistersGearResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme returned error: %v", err)
	}

	for _, kind := range []string{"Ability", "Mandate", "GovernedAction", "EscalationItem"} {
		obj, err := scheme.New(GroupVersion.WithKind(kind))
		if err != nil {
			t.Fatalf("expected %s to be registered: %v", kind, err)
		}
		if obj == nil {
			t.Fatalf("expected non-nil object for %s", kind)
		}
	}
}

func TestAbilityDeepCopyCopiesNestedMaps(t *testing.T) {
	ability := &Ability{
		Spec: AbilitySpec{
			Reversibility: map[string]string{"RECORD_ANNOTATE": "reversible"},
		},
	}

	copy := ability.DeepCopy()
	copy.Spec.Reversibility["RECORD_ANNOTATE"] = "changed"

	if ability.Spec.Reversibility["RECORD_ANNOTATE"] != "reversible" {
		t.Fatal("expected DeepCopy to isolate nested maps")
	}
}
