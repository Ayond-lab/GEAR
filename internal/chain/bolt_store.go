package chain

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	bolt "go.etcd.io/bbolt"
)

var entriesBucket = []byte("entries")

type BoltStore struct {
	db *bolt.DB
}

func OpenBoltStore(path string) (*BoltStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	return &BoltStore{db: db}, nil
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}

func (s *BoltStore) Append(ctx context.Context, entry Entry) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}

	var stored Entry
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(entriesBucket)
		if err != nil {
			return err
		}

		prev, err := lastEntry(bucket)
		if err != nil {
			return err
		}
		if entry.TS == "" {
			entry.TS = time.Now().UTC().Format(time.RFC3339Nano)
		}
		stored, err = Append(prev, entry)
		if err != nil {
			return err
		}

		data, err := json.Marshal(stored)
		if err != nil {
			return err
		}
		return bucket.Put(sequenceKey(stored.Seq), data)
	})
	if err != nil {
		return Entry{}, err
	}
	return stored, nil
}

func (s *BoltStore) Entries(ctx context.Context) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var entries []Entry
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(entriesBucket)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(_, value []byte) error {
			var entry Entry
			if err := json.Unmarshal(value, &entry); err != nil {
				return err
			}
			entries = append(entries, entry)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *BoltStore) Verify(ctx context.Context) (Verification, error) {
	entries, err := s.Entries(ctx)
	if err != nil {
		return Verification{}, err
	}
	return Verify(entries), nil
}

func (s *BoltStore) EffectsWithoutDecisions(ctx context.Context) ([]string, error) {
	entries, err := s.Entries(ctx)
	if err != nil {
		return nil, err
	}

	decisions := make(map[string]bool)
	for _, entry := range entries {
		if entry.Type == "decision" && entry.ActionRef != "" {
			decisions[entry.ActionRef] = true
		}
	}

	missingSet := make(map[string]bool)
	for _, entry := range entries {
		if entry.Type != "effect" || entry.ActionRef == "" {
			continue
		}
		if !decisions[entry.ActionRef] {
			missingSet[entry.ActionRef] = true
		}
	}

	missing := make([]string, 0, len(missingSet))
	for actionRef := range missingSet {
		missing = append(missing, actionRef)
	}
	sort.Strings(missing)
	return missing, nil
}

func lastEntry(bucket *bolt.Bucket) (Entry, error) {
	_, value := bucket.Cursor().Last()
	if value == nil {
		return Entry{}, nil
	}
	var entry Entry
	if err := json.Unmarshal(value, &entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

func sequenceKey(seq uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, seq)
	return key
}
