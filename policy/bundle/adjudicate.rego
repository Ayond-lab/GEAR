package gear.adjudicate

import rego.v1

required_fields := {
	"actionClass",
	"abilityRef",
	"abilityVersion",
	"mandateRef",
	"mandateVersion",
	"confidence",
	"dataClasses",
	"reversibility",
	"counters",
	"payloadDigest",
}

input_fields := {field | input[field]}
extra_fields := {field | input[field]; not required_fields[field]}
missing_fields := {field | required_fields[field]; not input[field]}

valid_field_set if {
	count(input_fields) == 10
	count(extra_fields) == 0
	count(missing_fields) == 0
}

default decision := {
	"decision": "deny",
	"ruleFired": {"id": "R-DEFAULT-DENY", "version": 1},
	"reason": "default deny",
}

decision := {
	"decision": "deny",
	"ruleFired": {"id": "R-INPUT-FIELDS", "version": 1},
	"reason": "decision input must contain exactly the fixed ten fields",
} if {
	not valid_field_set
}

decision := {
	"decision": "deny",
	"ruleFired": {"id": "D1", "version": 1},
	"reason": "actionClass CANDIDATE_RANK forbidden by mandate clause D1",
} if {
	valid_field_set
	input.actionClass == "CANDIDATE_RANK"
}

decision := {
	"decision": "escalate",
	"ruleFired": {"id": "R-CONFIDENCE-LOW", "version": 1},
	"reason": "confidence below mandate threshold",
} if {
	valid_field_set
	input.actionClass == "RECORD_ANNOTATE"
	input.confidence < "0.70"
}

decision := {
	"decision": "authorise",
	"ruleFired": {"id": "R-PERMIT", "version": 1},
	"reason": "all validations passed and mandate permits action",
} if {
	valid_field_set
	input.actionClass == "RECORD_ANNOTATE"
	input.confidence >= "0.70"
}

