package chain

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestBoltStoreAppendReopenVerifyFiveHundredEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.db")
	store := openTestBoltStore(t, path)
	appendTestEntries(t, store, 500)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openTestBoltStore(t, path)
	defer reopened.Close()

	entries, err := reopened.Entries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 500 {
		t.Fatalf("expected 500 durable entries, got %d", len(entries))
	}

	result, err := reopened.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("expected durable chain to verify, affected=%#v", result.Affected)
	}
}

func TestBoltStoreVerifyDetectsModification(t *testing.T) {
	path := buildTamperStore(t)
	tamperBoltEntry(t, path, func(bucket *bolt.Bucket) {
		entry := readBoltEntry(t, bucket, 250)
		entry.Decision = "deny"
		writeBoltEntry(t, bucket, 250, entry)
	})

	store := openTestBoltStore(t, path)
	defer store.Close()
	result, err := store.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !containsSeq(result.Affected, 250) {
		t.Fatalf("expected modification around seq 250 to be detected, got %#v", result)
	}
}

func TestBoltStoreVerifyDetectsDeletion(t *testing.T) {
	path := buildTamperStore(t)
	tamperBoltEntry(t, path, func(bucket *bolt.Bucket) {
		if err := bucket.Delete(sequenceKey(250)); err != nil {
			t.Fatal(err)
		}
	})

	store := openTestBoltStore(t, path)
	defer store.Close()
	result, err := store.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !containsSeq(result.Affected, 250) {
		t.Fatalf("expected deletion around seq 250 to be detected, got %#v", result)
	}
}

func TestBoltStoreVerifyDetectsReordering(t *testing.T) {
	path := buildTamperStore(t)
	tamperBoltEntry(t, path, func(bucket *bolt.Bucket) {
		left := readBoltEntry(t, bucket, 250)
		right := readBoltEntry(t, bucket, 251)
		writeBoltEntry(t, bucket, 250, right)
		writeBoltEntry(t, bucket, 251, left)
	})

	store := openTestBoltStore(t, path)
	defer store.Close()
	result, err := store.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !containsSeq(result.Affected, 250) {
		t.Fatalf("expected reordering around seq 250 to be detected, got %#v", result)
	}
}

func TestBoltStoreReportsEffectsWithoutDecisions(t *testing.T) {
	store := openTestBoltStore(t, filepath.Join(t.TempDir(), "audit.db"))
	defer store.Close()

	appendEntry(t, store, Entry{
		Type:      "effect",
		ActionRef: "ga-missing",
		Actor:     "gear-pep",
		Mandate:   "MND-2026-021:2",
		Rule:      "R-PERMIT:1",
		Decision:  "authorise",
	})
	appendEntry(t, store, Entry{
		Type:      "decision",
		ActionRef: "ga-present",
		Actor:     "gear-policy",
		Mandate:   "MND-2026-021:2",
		Rule:      "R-PERMIT:1",
		Decision:  "authorise",
	})
	appendEntry(t, store, Entry{
		Type:      "effect",
		ActionRef: "ga-present",
		Actor:     "gear-pep",
		Mandate:   "MND-2026-021:2",
		Rule:      "R-PERMIT:1",
		Decision:  "authorise",
	})

	missing, err := store.EffectsWithoutDecisions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != "ga-missing" {
		t.Fatalf("expected ga-missing to be reported, got %#v", missing)
	}
}

func buildTamperStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.db")
	store := openTestBoltStore(t, path)
	appendTestEntries(t, store, 500)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func appendTestEntries(t *testing.T, store *BoltStore, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		appendEntry(t, store, Entry{
			TS:           "2026-08-26T00:00:00Z",
			Type:         "decision",
			ActionRef:    "ga-a7",
			Actor:        "gear-policy",
			Mandate:      "MND-2026-021:2",
			Rule:         "R-PERMIT:1",
			Decision:     "authorise",
			InputsDigest: "sha256:inputs",
			Model:        "none",
			DataAccessed: []string{"sha256:payload"},
		})
	}
}

func appendEntry(t *testing.T, store *BoltStore, entry Entry) Entry {
	t.Helper()
	stored, err := store.Append(context.Background(), entry)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func openTestBoltStore(t *testing.T, path string) *BoltStore {
	t.Helper()
	store, err := OpenBoltStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func tamperBoltEntry(t *testing.T, path string, tamper func(bucket *bolt.Bucket)) {
	t.Helper()
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(entriesBucket)
		if bucket == nil {
			t.Fatal("missing entries bucket")
		}
		tamper(bucket)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func readBoltEntry(t *testing.T, bucket *bolt.Bucket, seq uint64) Entry {
	t.Helper()
	raw := bucket.Get(sequenceKey(seq))
	if raw == nil {
		t.Fatalf("missing seq %d", seq)
	}
	var entry Entry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

func writeBoltEntry(t *testing.T, bucket *bolt.Bucket, seq uint64, entry Entry) {
	t.Helper()
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := bucket.Put(sequenceKey(seq), data); err != nil {
		t.Fatal(err)
	}
}

func containsSeq(ranges []Range, seq uint64) bool {
	for _, affected := range ranges {
		if affected.Start <= seq && seq <= affected.End {
			return true
		}
	}
	return false
}
