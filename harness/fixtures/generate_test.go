package main

import "testing"

func TestGenerateDistribution(t *testing.T) {
	applications := generate()
	if len(applications) != 60 {
		t.Fatalf("expected 60 applications, got %d", len(applications))
	}

	counts := map[string]int{}
	injections := 0
	for _, application := range applications {
		counts[application.Status]++
		if application.InjectionCase {
			injections++
		}
	}

	expected := map[string]int{
		"EEA national":          34,
		"Holds permit":          11,
		"Requires sponsorship":  12,
		"Unclear":               3,
	}
	for status, count := range expected {
		if counts[status] != count {
			t.Fatalf("status %q: expected %d, got %d", status, count, counts[status])
		}
	}
	if injections != 3 {
		t.Fatalf("expected 3 prompt-injection controls, got %d", injections)
	}
}

