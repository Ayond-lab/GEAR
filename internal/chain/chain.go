package chain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const GenesisHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

type Entry struct {
	Seq          uint64   `json:"seq"`
	PrevHash     string   `json:"prevHash"`
	Hash         string   `json:"hash"`
	TS           string   `json:"ts"`
	Type         string   `json:"type"`
	ActionRef    string   `json:"actionRef"`
	Actor        string   `json:"actor"`
	Mandate      string   `json:"mandate"`
	Rule         string   `json:"rule"`
	Decision     string   `json:"decision"`
	InputsDigest string   `json:"inputsDigest"`
	Model        string   `json:"model"`
	DataAccessed []string `json:"dataAccessed"`
}

type Range struct {
	Start uint64
	End   uint64
}

type Verification struct {
	OK       bool
	Affected []Range
}

func Append(prev Entry, next Entry) (Entry, error) {
	if next.Seq == 0 {
		next.Seq = prev.Seq + 1
	}
	if next.PrevHash == "" {
		if prev.Hash == "" {
			next.PrevHash = GenesisHash
		} else {
			next.PrevHash = prev.Hash
		}
	}
	hash, err := HashEntry(next)
	if err != nil {
		return Entry{}, err
	}
	next.Hash = hash
	return next, nil
}

func HashEntry(entry Entry) (string, error) {
	entry.Hash = ""
	canonical, err := CanonicalJSON(entry)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append(canonical, []byte(entry.PrevHash)...))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func CanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buffer.Bytes()), nil
}

func Verify(entries []Entry) Verification {
	if len(entries) == 0 {
		return Verification{OK: true}
	}

	var affected []Range
	prevHash := GenesisHash
	expectedSeq := uint64(1)

	for _, entry := range entries {
		if entry.Seq != expectedSeq {
			affected = append(affected, Range{Start: expectedSeq, End: entry.Seq})
			expectedSeq = entry.Seq
		}
		if entry.PrevHash != prevHash {
			affected = append(affected, Range{Start: entry.Seq, End: entry.Seq})
		}
		expectedHash, err := HashEntry(entry)
		if err != nil || entry.Hash != expectedHash {
			affected = append(affected, Range{Start: entry.Seq, End: entry.Seq})
		}
		prevHash = entry.Hash
		expectedSeq++
	}

	return Verification{OK: len(affected) == 0, Affected: coalesce(affected)}
}

func Ref(seq uint64) string {
	return fmt.Sprintf("ae-%07d", seq)
}

func coalesce(ranges []Range) []Range {
	if len(ranges) < 2 {
		return ranges
	}
	out := []Range{ranges[0]}
	for _, current := range ranges[1:] {
		last := &out[len(out)-1]
		if current.Start <= last.End+1 {
			if current.End > last.End {
				last.End = current.End
			}
			continue
		}
		out = append(out, current)
	}
	return out
}

