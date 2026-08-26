# PEP Loopback Contract

Base address:

```text
http://127.0.0.1:9191
```

Required endpoints:

- `POST /v1/extract`
- `POST /v1/effects`
- `GET /v1/healthz`

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

