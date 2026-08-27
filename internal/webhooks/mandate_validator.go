package webhooks

import (
	"context"
	"errors"
	"fmt"

	gearv1 "gear/api/v1"
	"gear/internal/subsume"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	ErrNilReader        = errors.New("mandate validator requires a Kubernetes reader")
	ErrNilMandate       = errors.New("mandate validator requires a mandate")
	ErrUnexpectedObject = errors.New("mandate validator received an unexpected object type")
)

type MandateValidator struct {
	Reader client.Reader
}

func NewMandateValidator(reader client.Reader) MandateValidator {
	return MandateValidator{Reader: reader}
}

func (v MandateValidator) ValidateCreateMandate(ctx context.Context, mandate *gearv1.Mandate) error {
	return v.validateMandate(ctx, mandate)
}

func (v MandateValidator) ValidateUpdateMandate(ctx context.Context, _ *gearv1.Mandate, mandate *gearv1.Mandate) error {
	return v.validateMandate(ctx, mandate)
}

func (v MandateValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	mandate, ok := obj.(*gearv1.Mandate)
	if !ok {
		return nil, fmt.Errorf("%w: %T", ErrUnexpectedObject, obj)
	}
	return nil, v.ValidateCreateMandate(ctx, mandate)
}

func (v MandateValidator) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	oldMandate, ok := oldObj.(*gearv1.Mandate)
	if !ok {
		return nil, fmt.Errorf("%w: old=%T", ErrUnexpectedObject, oldObj)
	}
	newMandate, ok := newObj.(*gearv1.Mandate)
	if !ok {
		return nil, fmt.Errorf("%w: new=%T", ErrUnexpectedObject, newObj)
	}
	return nil, v.ValidateUpdateMandate(ctx, oldMandate, newMandate)
}

func (v MandateValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func ValidateMandateSubsumption(ability gearv1.AbilitySpec, mandate gearv1.MandateSpec) error {
	result := subsume.Check(ability, mandate)
	if result.OK() {
		return nil
	}
	return invalidMandate(mandate.MandateID, fieldErrorsForViolations(result.Violations))
}

func (v MandateValidator) validateMandate(ctx context.Context, mandate *gearv1.Mandate) error {
	if mandate == nil {
		return ErrNilMandate
	}
	if v.Reader == nil {
		return ErrNilReader
	}

	var ability gearv1.Ability
	key := types.NamespacedName{Name: mandate.Spec.AbilityRef, Namespace: mandate.Namespace}
	if key.Name == "" {
		return invalidMandate(mandate.Name, field.ErrorList{
			field.Required(field.NewPath("spec").Child("abilityRef"), "mandate must reference a known ability"),
		})
	}
	if err := v.Reader.Get(ctx, key, &ability); err != nil {
		if apierrors.IsNotFound(err) {
			return invalidMandate(mandate.Name, field.ErrorList{
				field.NotFound(field.NewPath("spec").Child("abilityRef"), key.String()),
			})
		}
		return fmt.Errorf("resolve referenced ability %s: %w", key.String(), err)
	}

	return ValidateMandateSubsumption(ability.Spec, mandate.Spec)
}

func invalidMandate(name string, fieldErrors field.ErrorList) error {
	return apierrors.NewInvalid(
		schema.GroupKind{Group: gearv1.GroupVersion.Group, Kind: "Mandate"},
		name,
		fieldErrors,
	)
}

func fieldErrorsForViolations(violations []subsume.Violation) field.ErrorList {
	errs := make(field.ErrorList, 0, len(violations))
	for _, violation := range violations {
		errs = append(errs, field.Invalid(pathForViolation(violation.Field), violation.Value, violation.Reason))
	}
	return errs
}

func pathForViolation(name string) *field.Path {
	switch name {
	case "abilityVersion":
		return field.NewPath("spec").Child("abilityVersion")
	case "certificationStatus":
		return field.NewPath("spec").Child("abilityRef")
	case "sources":
		return field.NewPath("spec").Child("sources")
	case "connectorGrants":
		return field.NewPath("spec").Child("connectorGrants")
	case "actionGrants.disposition":
		return field.NewPath("spec").Child("actionGrants").Child("disposition")
	case "actionGrants.class":
		return field.NewPath("spec").Child("actionGrants").Child("class")
	case "reversibilityClasses":
		return field.NewPath("spec").Child("actionGrants")
	case "caps.dailyActions":
		return field.NewPath("spec").Child("caps").Child("dailyActions")
	default:
		return field.NewPath("spec").Child(name)
	}
}
