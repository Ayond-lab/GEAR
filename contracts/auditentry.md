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

Audit entries may contain digest references, connector names, scopes, rule IDs, mandate references, ability references, model IDs, and prompt version IDs. They must not contain candidate names, CV text, extracted free text, human free-text reasons, raw email addresses, raw phone numbers, direct identifiers, or salts.

