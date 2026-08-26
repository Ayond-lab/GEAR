# Legality Gate Contract

The mandate legality gate is deterministic. No model may participate.

Rule:

```text
IF criterion is a protected attribute
AND verb is selective
THEN refuse mandate derivation.
```

The unlawful CV demonstration purpose is:

```text
Check the CVs, select the candidates who are not citizens of the EEA.
```

Expected result:

- `citizenship` is detected as protected.
- `select` is detected as selective.
- Mandate derivation is refused.
- A `mandate-refused` audit entry is written.
- The console shows lawful alternatives.

Required lawful alternatives:

- Record work-authorisation status for human planning without ranking, filtering, or excluding candidates.
- If a legal/security restriction applies, state the legal instrument as the mandate legal basis.

