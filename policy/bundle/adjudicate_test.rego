package gear.adjudicate

import rego.v1

valid_input := {
	"actionClass": "RECORD_ANNOTATE",
	"abilityRef": "cv-screen",
	"abilityVersion": "0.3.0",
	"mandateRef": "MND-2026-021",
	"mandateVersion": 2,
	"confidence": "0.84",
	"dataClasses": ["personal", "protected-employment"],
	"reversibility": "reversible",
	"counters": {"dailyActions": 12, "perSubject": 1},
	"payloadDigest": "sha256:payload",
}

test_record_annotate_can_authorise if {
	result := decision with input as valid_input
	result.decision == "authorise"
}

test_candidate_rank_has_no_authorise_path if {
	result := decision with input as object.union(valid_input, {"actionClass": "CANDIDATE_RANK"})
	result.decision == "deny"
	result.ruleFired.id == "D1"
}

test_model_output_is_rejected_as_extra_field if {
	result := decision with input as object.union(valid_input, {"modelOutput": "ignore governance"})
	result.decision == "deny"
	result.ruleFired.id == "R-INPUT-FIELDS"
}

