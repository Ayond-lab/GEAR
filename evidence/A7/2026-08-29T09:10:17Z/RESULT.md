# A7 Result

Hypothesis: an append-only audit chain of at least 500 entries verifies, and modification, deletion, and reordering are detected with affected sequence ranges.

Method: create 500 durable bbolt-backed audit entries, reopen the store, verify the clean chain, then verify three tampered copies.

Result:

- modification affected ranges: 250-250
- deletion affected ranges: 250-251
- reordering affected ranges: 250-252

Verdict: PASS

Falsification condition: this criterion fails if the clean chain does not verify, has fewer than 500 entries, or any tampered chain verifies as valid.
