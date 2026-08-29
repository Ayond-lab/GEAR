# Audit Entry Contract

GEAR writes append-only audit entries before decisions are returned and after authorised effects execute.

Required logical fields:

- `seq`
- `prevHash`
- `hash`
- `ts`
- `type`
- `actionRef`
- `actor`
- `mandate`
- `rule`
- `decision`
- `inputsDigest`
- `model`
- `dataAccessed`

Hash rule:

```text
hash = SHA256(JCS(entry without hash) || prevHash)
```

The TRL 4 implementation stores the chain in single-replica durable storage. Production high availability is deferred.

## Development HTTP API

`gear-audit` exposes the following lab endpoints:

- `POST /v1/entries`: append one audit entry, commit it durably, and return `auditRef` plus the stored entry.
- `GET /v1/entries`: list stored entries in sequence order.
- `GET /v1/verify`: verify the full hash chain and report affected ranges.
- `GET /v1/reconcile/effects-without-decisions`: report effect entries that have no preceding decision for the same action reference.

Audit entries may contain digest references, connector names, scopes, rule IDs, mandate references, ability references, model IDs, and prompt version IDs. They must not contain candidate names, CV text, extracted free text, human free-text reasons, raw email addresses, raw phone numbers, direct identifiers, or salts.
