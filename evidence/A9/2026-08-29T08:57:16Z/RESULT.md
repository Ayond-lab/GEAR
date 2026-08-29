# A9 Result

Hypothesis: when gear-audit is unavailable, every adjudication fails closed with deny and no execution token.

Method: run the gear-policy HTTP adjudication handler with its audit client pointed at a stopped local endpoint, then submit action requests that would otherwise authorise or escalate.

Result: all tested adjudications returned deny with rule R-AUDIT-UNAVAILABLE, no audit reference, no token, and no escalation reference.

Verdict: PASS

Falsification condition: this criterion fails if any adjudication authorises, escalates, returns a token, or records a non-empty audit reference while audit is unavailable.
