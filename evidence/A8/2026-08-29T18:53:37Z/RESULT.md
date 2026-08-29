# A8 Policy Latency

Hypothesis: `gear-policy` p95 latency is recorded over at least 200 trials while `gear-inference` is under load.

Method: run 200 deterministic policy adjudications against the in-process policy core while four concurrent workers exercise the synthetic work-authorisation extractor.

Result: trials=200 p95Micros=364 inferenceIterations=48061.

Verdict: PASS

Falsification condition: A8 fails if fewer than 200 trials run, inference load is inactive, p95 is not recorded, or any policy decision lacks durable audit evidence.
