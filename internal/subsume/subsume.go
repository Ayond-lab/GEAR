package subsume

import (
	"errors"
	"fmt"
	"math/big"

	gearv1 "gear/api/v1"
)

var ErrWidened = errors.New("mandate widens referenced ability manifest")

type Violation struct {
	Field  string
	Value  string
	Reason string
}

type Result struct {
	Violations []Violation
}

func (r Result) OK() bool {
	return len(r.Violations) == 0
}

func (r Result) Error() error {
	if r.OK() {
		return nil
	}
	return fmt.Errorf("%w: %s=%s (%s)", ErrWidened, r.Violations[0].Field, r.Violations[0].Value, r.Violations[0].Reason)
}

func Check(ability gearv1.AbilitySpec, mandate gearv1.MandateSpec) Result {
	var violations []Violation

	if mandate.AbilityVersion != ability.Version {
		violations = append(violations, Violation{
			Field:  "abilityVersion",
			Value:  mandate.AbilityVersion,
			Reason: "mandate references a different ability version",
		})
	}

	if ability.Certification == "revoked" {
		violations = append(violations, Violation{
			Field:  "certificationStatus",
			Value:  ability.Certification,
			Reason: "revoked abilities cannot be mandated",
		})
	}

	triggerSet := makeSetPairs(ability.DeclaredTriggers)
	for _, source := range mandate.Sources {
		key := source.Type + ":" + source.ID
		if !triggerSet[key] {
			violations = append(violations, Violation{Field: "sources", Value: key, Reason: "source is outside declared triggers"})
		}
	}

	scopeSet := makeConnectorSet(ability.ConnectorScopes)
	for _, grant := range mandate.ConnectorGrants {
		key := grant.Connector + ":" + grant.Scope
		if !scopeSet[key] {
			violations = append(violations, Violation{Field: "connectorGrants", Value: key, Reason: "connector grant is outside manifest scopes"})
		}
	}

	actionSet := makeStringSet(ability.ActionClasses)
	for _, grant := range mandate.ActionGrants {
		if !validDisposition(grant.Disposition) {
			violations = append(violations, Violation{Field: "actionGrants.disposition", Value: grant.Disposition, Reason: "invalid disposition"})
		}
		if !actionSet[grant.Class] {
			violations = append(violations, Violation{Field: "actionGrants.class", Value: grant.Class, Reason: "action class is outside manifest"})
		}
		if _, ok := ability.Reversibility[grant.Class]; !ok && actionSet[grant.Class] {
			violations = append(violations, Violation{Field: "reversibilityClasses", Value: grant.Class, Reason: "manifest lacks reversibility for action class"})
		}
	}

	if ability.Ceilings.DailyActions > 0 && mandate.Caps.DailyActions > ability.Ceilings.DailyActions {
		violations = append(violations, Violation{Field: "caps.dailyActions", Value: fmt.Sprint(mandate.Caps.DailyActions), Reason: "cap exceeds manifest ceiling"})
	}

	for key, value := range mandate.Thresholds {
		if !isDecimalString(value) {
			violations = append(violations, Violation{Field: "thresholds." + key, Value: value, Reason: "threshold is not a decimal string"})
		}
	}

	return Result{Violations: violations}
}

func makeSetPairs(items []gearv1.TriggerDecl) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item.Type+":"+item.ID] = true
	}
	return out
}

func makeConnectorSet(items []gearv1.ConnectorScope) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item.Connector+":"+item.Scope] = true
	}
	return out
}

func makeStringSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func validDisposition(value string) bool {
	switch value {
	case "permit", "escalate", "forbid":
		return true
	default:
		return false
	}
}

func isDecimalString(value string) bool {
	if value == "" {
		return false
	}
	_, ok := new(big.Rat).SetString(value)
	return ok
}

