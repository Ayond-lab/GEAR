# A2 Candidate Rank Denial

Hypothesis: under `MND-2026-021` v2, `CANDIDATE_RANK` returns deny, cites D1, produces no effect, and is recorded.

Method: submit a synthetic `CANDIDATE_RANK` effect intent through the in-process PEP, policy adjudicator, and append-only audit chain.

Result: PASS

Verdict: PASS

Falsification condition: A2 fails if ranking authorises or escalates, omits D1, writes an effect, or fails audit-chain verification.
