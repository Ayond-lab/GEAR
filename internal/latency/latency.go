package latency

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"gear/internal/chain"
	"gear/internal/cvdemo"
	"gear/internal/policy"
)

type Config struct {
	Trials           int
	InferenceWorkers int
	Now              func() time.Time
}

type Bucket struct {
	UpperBoundMicros int `json:"upperBoundMicros"`
	Count            int `json:"count"`
}

type Result struct {
	Trials              int                    `json:"trials"`
	InferenceWorkers    int                    `json:"inferenceWorkers"`
	InferenceIterations int64                  `json:"inferenceIterations"`
	MinMicros           int64                  `json:"minMicros"`
	P50Micros           int64                  `json:"p50Micros"`
	P95Micros           int64                  `json:"p95Micros"`
	MaxMicros           int64                  `json:"maxMicros"`
	Histogram           []Bucket               `json:"histogram"`
	Decisions           map[string]int         `json:"decisions"`
	AuditEntries        int                    `json:"auditEntries"`
	ChainVerification   chain.Verification     `json:"chainVerification"`
	LoadProfile         map[string]interface{} `json:"loadProfile"`
}

func Run(ctx context.Context, config Config) (Result, error) {
	trials := config.Trials
	if trials <= 0 {
		trials = 200
	}
	workers := config.InferenceWorkers
	if workers <= 0 {
		workers = 4
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	loadCtx, cancelLoad := context.WithCancel(ctx)
	var wg sync.WaitGroup
	var inferenceIterations atomic.Int64
	applications := cvdemo.GenerateApplications()
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for i := offset; ; i++ {
				select {
				case <-loadCtx.Done():
					return
				default:
					_ = cvdemo.ExtractWorkAuthorisation(applications[i%len(applications)])
					inferenceIterations.Add(1)
				}
			}
		}(worker)
	}

	audit := &cvdemo.MemoryAudit{Now: now}
	adjudicator := policy.NewAdjudicator(policy.DefaultCVRuntimePolicy(), audit)
	durations := make([]int64, 0, trials)
	decisions := map[string]int{}
	for trial := 0; trial < trials; trial++ {
		select {
		case <-ctx.Done():
			cancelLoad()
			wg.Wait()
			return Result{}, ctx.Err()
		default:
		}
		input := policy.DecisionInput{
			ActionClass:    "RECORD_ANNOTATE",
			AbilityRef:     cvdemo.AbilityRef,
			AbilityVersion: cvdemo.AbilityVersion,
			MandateRef:     cvdemo.MandateRef,
			MandateVersion: cvdemo.MandateVersion,
			Confidence:     "0.84",
			DataClasses:    []string{"personal", "protected-employment"},
			Reversibility:  "reversible",
			Counters:       map[string]int{"dailyActions": 12 + trial, "perSubject": 1},
			PayloadDigest:  applications[trial%len(applications)].PayloadDigest,
		}
		data, err := json.Marshal(input)
		if err != nil {
			cancelLoad()
			wg.Wait()
			return Result{}, err
		}
		started := time.Now()
		response := adjudicator.Adjudicate(ctx, data)
		durations = append(durations, time.Since(started).Microseconds())
		decisions[string(response.Decision)]++
		if response.Decision != policy.Authorise || response.AuditRef == "" || response.Token == nil {
			cancelLoad()
			wg.Wait()
			return Result{}, fmt.Errorf("trial %d did not authorise with durable audit and token: %#v", trial, response)
		}
	}
	cancelLoad()
	wg.Wait()

	entries := audit.Snapshot()
	sortedDurations := append([]int64(nil), durations...)
	sort.Slice(sortedDurations, func(i, j int) bool { return sortedDurations[i] < sortedDurations[j] })
	result := Result{
		Trials:              trials,
		InferenceWorkers:    workers,
		InferenceIterations: inferenceIterations.Load(),
		MinMicros:           sortedDurations[0],
		P50Micros:           percentile(sortedDurations, 0.50),
		P95Micros:           percentile(sortedDurations, 0.95),
		MaxMicros:           sortedDurations[len(sortedDurations)-1],
		Histogram:           histogram(durations),
		Decisions:           decisions,
		AuditEntries:        len(entries),
		ChainVerification:   chain.Verify(entries),
		LoadProfile: map[string]interface{}{
			"inferenceService": "deterministic synthetic extractor",
			"workers":          workers,
			"fixtureCount":     len(applications),
			"policyTrials":     trials,
		},
	}
	if !result.ChainVerification.OK {
		return Result{}, fmt.Errorf("latency run audit chain did not verify")
	}
	return result, nil
}

func percentile(sorted []int64, q float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	index := int(q*float64(len(sorted)-1) + 0.999999)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func histogram(values []int64) []Bucket {
	bounds := []int64{100, 250, 500, 1000, 2500, 5000, 10000, 25000, 50000}
	buckets := make([]Bucket, len(bounds)+1)
	for i, bound := range bounds {
		buckets[i].UpperBoundMicros = int(bound)
	}
	buckets[len(buckets)-1].UpperBoundMicros = -1
	for _, value := range values {
		placed := false
		for i, bound := range bounds {
			if value <= bound {
				buckets[i].Count++
				placed = true
				break
			}
		}
		if !placed {
			buckets[len(buckets)-1].Count++
		}
	}
	return buckets
}
