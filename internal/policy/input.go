package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
)

var ErrInvalidDecisionInput = errors.New("invalid decision input")

var requiredDecisionFields = []string{
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

type DecisionInput struct {
	ActionClass    string         `json:"actionClass"`
	AbilityRef     string         `json:"abilityRef"`
	AbilityVersion string         `json:"abilityVersion"`
	MandateRef     string         `json:"mandateRef"`
	MandateVersion int            `json:"mandateVersion"`
	Confidence     string         `json:"confidence"`
	DataClasses    []string       `json:"dataClasses"`
	Reversibility  string         `json:"reversibility"`
	Counters       map[string]int `json:"counters"`
	PayloadDigest  string         `json:"payloadDigest"`
}

func RequiredDecisionFields() []string {
	return append([]string(nil), requiredDecisionFields...)
}

func DecodeExactDecisionInput(data []byte) (DecisionInput, error) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return DecisionInput{}, fmt.Errorf("%w: malformed json: %v", ErrInvalidDecisionInput, err)
	}

	if err := validateFieldSet(raw); err != nil {
		return DecisionInput{}, err
	}

	var input DecisionInput
	if err := json.Unmarshal(data, &input); err != nil {
		return DecisionInput{}, fmt.Errorf("%w: %v", ErrInvalidDecisionInput, err)
	}
	if !IsDecimalString(input.Confidence) {
		return DecisionInput{}, fmt.Errorf("%w: confidence is not a decimal string", ErrInvalidDecisionInput)
	}
	return input, nil
}

func validateFieldSet(raw map[string]json.RawMessage) error {
	required := make(map[string]bool, len(requiredDecisionFields))
	for _, field := range requiredDecisionFields {
		required[field] = true
	}

	var missing []string
	for _, field := range requiredDecisionFields {
		if _, ok := raw[field]; !ok {
			missing = append(missing, field)
		}
	}

	var extra []string
	for field := range raw {
		if !required[field] {
			extra = append(extra, field)
		}
	}

	if len(missing) > 0 || len(extra) > 0 {
		sort.Strings(missing)
		sort.Strings(extra)
		return fmt.Errorf("%w: expected exactly ten fields, missing=%v extra=%v", ErrInvalidDecisionInput, missing, extra)
	}
	return nil
}

func IsDecimalString(value string) bool {
	if value == "" {
		return false
	}
	_, ok := new(big.Rat).SetString(value)
	return ok
}

func CompareDecimalStrings(left, right string) (int, error) {
	l, ok := new(big.Rat).SetString(left)
	if !ok {
		return 0, fmt.Errorf("%w: invalid decimal %q", ErrInvalidDecisionInput, left)
	}
	r, ok := new(big.Rat).SetString(right)
	if !ok {
		return 0, fmt.Errorf("%w: invalid decimal %q", ErrInvalidDecisionInput, right)
	}
	return l.Cmp(r), nil
}

