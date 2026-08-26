# Evidence

Every experiment writes retained evidence under:

```text
evidence/<ID>/<RFC3339 timestamp>/
```

Each run directory must include:

- `RESULT.md`
- `ENV.md`
- Raw artefacts relevant to the acceptance criterion

Failures are retained.

