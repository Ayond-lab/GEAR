# A5 Admission Rejection Experiment

## Hypothesis

A signed mandate that grants `CANDIDATE_RANK: permit` for the protected/selective citizenship purpose is rejected by Kubernetes admission.

## Method

The harness used the active cluster-admin context to run `kubectl apply` against the invalid signed mandate fixture and captured webhook logs.

## Result

PASS

```text
The Mandate "MND-CANDIDATE-RANK-PERMIT" is invalid: 
* spec.purposeStatement: Invalid value: "sha256:0a0cb01ead785682": purpose refused by legality gate: protected criterion combined with selective verb
* spec.actionGrants: Invalid value: "CANDIDATE_RANK:permit": action class CANDIDATE_RANK was refused by legality gate for protected-attribute selective use
```

## Verdict

PASS

## Falsification Condition

A5 is falsified if `kubectl apply` succeeds or if the rejection is not caused by the legality-gate refusal for `CANDIDATE_RANK`.
