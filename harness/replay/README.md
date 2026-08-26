# Replay Harness

Replay tests validate idempotency keys and execution-token single-use behavior.

Idempotency key:

```text
sha256(sourceEventID || actionClass || payloadDigest)
```

