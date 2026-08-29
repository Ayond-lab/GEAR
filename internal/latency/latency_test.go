package latency

import (
	"context"
	"testing"
	"time"
)

func TestRunRecordsLatencyUnderInferenceLoad(t *testing.T) {
	result, err := Run(context.Background(), Config{
		Trials:           20,
		InferenceWorkers: 2,
		Now:              func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Trials != 20 || result.InferenceWorkers != 2 || result.InferenceIterations == 0 {
		t.Fatalf("unexpected load summary %#v", result)
	}
	if result.Decisions["authorise"] != 20 || result.AuditEntries != 20 {
		t.Fatalf("expected 20 authorised audited decisions, got decisions=%#v audit=%d", result.Decisions, result.AuditEntries)
	}
	if !result.ChainVerification.OK {
		t.Fatalf("expected verified audit chain, got %#v", result.ChainVerification)
	}
	if len(result.Histogram) == 0 || result.P95Micros < 0 || result.MaxMicros < result.MinMicros {
		t.Fatalf("unexpected latency metrics %#v", result)
	}
}

func TestRunDefaultsToA8TrialCount(t *testing.T) {
	result, err := Run(context.Background(), Config{InferenceWorkers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Trials != 200 {
		t.Fatalf("expected default 200 trials, got %d", result.Trials)
	}
}
