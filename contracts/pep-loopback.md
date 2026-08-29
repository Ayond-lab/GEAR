# PEP Loopback Contract

Base address:

```text
http://127.0.0.1:9191
```

Required endpoints:

- `POST /v1/extract`
- `POST /v1/effects`
- `GET /v1/healthz`

`gear-pep` binds this API to `127.0.0.1:9191` only. Startup rejects wildcard, pod-IP, service, or alternate-port listen addresses.

## `POST /v1/extract`

Request:

```json
{
  "sourceRef": "fixture://applications/0001",
  "payloadDigest": "sha256:...",
  "profile": "work-authorisation"
}
```

The request is accepted only when an active governed action is loaded and the payload digest matches that action. The endpoint returns structured extracted fields only; raw CV text and extracted free text are not returned to policy.

## `POST /v1/effects`

Request:

```json
{
  "actionClass": "RECORD_ANNOTATE",
  "connector": "candidate-record",
  "scope": "write",
  "payloadDigest": "sha256:...",
  "bodyDigest": "sha256:..."
}
```

The request is accepted only when action class and payload digest match the active governed action. Until the policy-backed mediator is configured, the endpoint returns a fail-closed `deny` and executes no effect.

The ability container calls only this loopback API. `gear-pep` mediates inference, adjudication, and effect execution. Connector credentials and mTLS client certificates are mounted only into `gear-pep`.

UID model:

- Ability container UID: `1001`
- `gear-pep` UID: `1337`

Required egress intent:

```text
Allow loopback.
Allow non-loopback egress only from UID 1337.
Reject all other non-loopback egress.
```

The hostile A6 negative control must prove egress fails with the init-container rules and succeeds when those rules are removed.
