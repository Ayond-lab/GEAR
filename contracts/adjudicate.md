# Adjudication Contract

Endpoint: `POST /v1/adjudicate`

Transport: mTLS. The development CA issues client certificates only to `gear-pep`.

## Request Boundary

The request body must contain exactly these ten fields:

1. `actionClass`
2. `abilityRef`
3. `abilityVersion`
4. `mandateRef`
5. `mandateVersion`
6. `confidence`
7. `dataClasses`
8. `reversibility`
9. `counters`
10. `payloadDigest`

No other fields are accepted. In particular, policy must reject request bodies containing model output, prompt text, extracted free text, CV text, or ability-supplied narrative.

`confidence` and threshold values are decimal strings and must be compared without `float64`.

## Response

The response contains a decision, rule reference, reason, audit reference, and optionally one execution token or one escalation reference.

Allowed decision values:

- `authorise`
- `deny`
- `escalate`

Default outcome is `deny`. Unavailability never grants permission.

