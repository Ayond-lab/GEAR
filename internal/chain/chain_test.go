package chain

import "testing"

func TestAppendAndVerifyFiveHundredEntries(t *testing.T) {
	entries := buildEntries(t, 500)
	result := Verify(entries)

	if !result.OK {
		t.Fatalf("expected chain to verify, affected=%#v", result.Affected)
	}
}

func TestVerifyDetectsModification(t *testing.T) {
	entries := buildEntries(t, 25)
	entries[12].Decision = "authorise"

	result := Verify(entries)

	if result.OK {
		t.Fatal("expected modification to be detected")
	}
}

func TestVerifyDetectsDeletion(t *testing.T) {
	entries := buildEntries(t, 25)
	entries = append(entries[:10], entries[11:]...)

	result := Verify(entries)

	if result.OK {
		t.Fatal("expected deletion to be detected")
	}
}

func TestVerifyDetectsReordering(t *testing.T) {
	entries := buildEntries(t, 25)
	entries[8], entries[9] = entries[9], entries[8]

	result := Verify(entries)

	if result.OK {
		t.Fatal("expected reordering to be detected")
	}
}

func TestRefUsesAuditEntryPrefix(t *testing.T) {
	if got := Ref(412); got != "ae-0000412" {
		t.Fatalf("unexpected ref %q", got)
	}
}

func buildEntries(t *testing.T, count int) []Entry {
	t.Helper()
	var prev Entry
	entries := make([]Entry, 0, count)
	for i := 0; i < count; i++ {
		next, err := Append(prev, Entry{
			TS:           "2026-08-26T00:00:00Z",
			Type:         "decision",
			ActionRef:    "ga-test",
			Actor:        "gear-policy",
			Mandate:      "MND-2026-021:2",
			Rule:         "R-PERMIT:1",
			Decision:     "authorise",
			InputsDigest: "sha256:inputs",
			Model:        "none",
			DataAccessed: []string{"sha256:payload"},
		})
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, next)
		prev = next
	}
	return entries
}

