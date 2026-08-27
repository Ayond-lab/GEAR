package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion identifies the Kubernetes API group and version used by GEAR.
var GroupVersion = schema.GroupVersion{Group: "gear.eu", Version: "v1"}

// SchemeBuilder registers GEAR resource types with a Kubernetes runtime scheme.
var SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

// AddToScheme adds all GEAR resource types to the supplied scheme.
var AddToScheme = SchemeBuilder.AddToScheme

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		GroupVersion,
		&Ability{},
		&AbilityList{},
		&Mandate{},
		&MandateList{},
		&GovernedAction{},
		&GovernedActionList{},
		&EscalationItem{},
		&EscalationItemList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
