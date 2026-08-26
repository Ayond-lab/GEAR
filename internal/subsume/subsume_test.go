package subsume

import (
	"errors"
	"testing"

	gearv1 "gear/api/v1"
)

func TestCheckAllowsNarrowedMandate(t *testing.T) {
	result := Check(cvScreenAbility(), narrowedMandate())

	if err := result.Error(); err != nil {
		t.Fatalf("expected narrowed mandate to pass, got %v", err)
	}
}

func TestCheckRejectsActionOutsideManifest(t *testing.T) {
	mandate := narrowedMandate()
	mandate.ActionGrants = append(mandate.ActionGrants, gearv1.ActionGrant{Class: "DELETE_RECORD", Disposition: "permit"})

	result := Check(cvScreenAbility(), mandate)

	if !errors.Is(result.Error(), ErrWidened) {
		t.Fatalf("expected widened error, got %v", result.Error())
	}
}

func TestCheckRejectsConnectorOutsideManifest(t *testing.T) {
	mandate := narrowedMandate()
	mandate.ConnectorGrants = append(mandate.ConnectorGrants, gearv1.ConnectorScope{Connector: "mail", Scope: "send"})

	result := Check(cvScreenAbility(), mandate)

	if result.OK() {
		t.Fatal("expected connector widening violation")
	}
}

func TestCheckRejectsCapsAboveCeiling(t *testing.T) {
	mandate := narrowedMandate()
	mandate.Caps.DailyActions = 501

	result := Check(cvScreenAbility(), mandate)

	if result.OK() {
		t.Fatal("expected cap widening violation")
	}
}

func cvScreenAbility() gearv1.AbilitySpec {
	return gearv1.AbilitySpec{
		Version:        "0.3.0",
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

func narrowedMandate() gearv1.MandateSpec {
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

