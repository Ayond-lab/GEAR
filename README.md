# GEAR: Governed Execution for Agentic Runtimes

GEAR is a governed execution layer for consequential automation. It separates what an agent is technically able to do from what it is authorised to do in a specific organisational context.

An ability performs work. GEAR decides whether a requested action is authorised, refused, or escalated to a human approver, and records decisions and effects in a tamper-evident audit chain.

This repository contains a laboratory development build using synthetic CV-screening data. It is intended to demonstrate the governed execution path, not to process real personal data or support live recruitment decisions.

## What This Repository Demonstrates

- Authority separation between ability manifest, mandate, and runtime decisioning.
- Mandate subsumption: a mandate may narrow an ability manifest, but may not widen it.
- Deterministic legality checks before mandate signing.
- A fixed ten-field policy input for runtime adjudication.
- Fail-closed decisioning when required services or evidence writes are unavailable.
- Sidecar enforcement through `gear-pep`.
- Tamper-evident audit entries using a hash chain.
- Human escalation for reserved or uncertain actions.
- Evidence capture for acceptance criteria A1-A10.
- A professional HR mandate demonstration UI.

## Demonstration Scenario

The demonstration domain is synthetic CV screening.

An HR user enters a natural-language mandate request through the console, for example:

```text
Rank the candidates who are citizens of EEA.
```

GEAR interprets the request as a selective action involving citizenship. The mandate is refused because the request combines a protected criterion with ranking. The console then presents a lawful alternative:

```text
Record work-authorisation status for human planning without ranking, filtering, or excluding candidates.
```

Under the narrowed mandate, the agent may record work-authorisation annotations for planning. It may not rank candidates by citizenship, contact candidates, or exceed the mandate limits. Unclear cases are escalated to a human reviewer, and governed decisions and effects are recorded without writing raw CV text or direct identifiers to the audit chain.

## Architecture

| Component | Role |
|---|---|
| `gear-webhooks` | Kubernetes admission control, subsumption validation, and sidecar injection |
| `gear-policy` | Deterministic policy adjudication |
| `gear-pep` | Local policy enforcement point for ability containers |
| `gear-audit` | Append-only tamper-evident audit chain |
| `gear-triggers` | Converts synthetic source events into governed actions |
| `gear-controllers` | Reconciles GEAR custom resources and execution state |
| `gear-mandate` | Mandate derivation, refusal, subsumption check, and signing |
| `gear-inference` | Synthetic CV work-authorisation extraction |
| `gear-console` | HR demonstration UI and evidence view |
| `gear-fixture-store` | Development-only synthetic fixture store |
| `harness` | Conformance tests, hostile experiments, replay, tamper tests, latency tests, and evidence capture |

## Governance Model

GEAR separates authority into three layers.

| Layer | Set by | Defines |
|---|---|---|
| Ability manifest | Ability publisher | What the ability is technically capable of doing |
| Mandate | Principal organisation | What the organisation permits for a specific ability version |
| Runtime | GEAR | Whether a specific action may proceed now |

A mandate may narrow the ability manifest, but it may never widen it. Runtime decisioning cannot grant authority that is absent from the signed ability manifest and mandate.

## Policy Boundary

`gear-policy` receives exactly ten decision fields:

- `actionClass`
- `abilityRef`
- `abilityVersion`
- `mandateRef`
- `mandateVersion`
- `confidence`
- `dataClasses`
- `reversibility`
- `counters`
- `payloadDigest`

Model output, prompt text, raw CV text, extracted free text, and ability-supplied explanations are excluded from policy inputs.

## Running Locally

Run the main local checks:

```bash
make test
make opa-test
make test-envtest
make conformance
```

Run the HR demonstration console:

```bash
GEAR_ADDR=127.0.0.1:18080 go run ./cmd/gear-console-api
```

Open:

```text
http://127.0.0.1:18080
```

For cluster-based validation, use the Kubernetes and evidence targets in the `Makefile`.

## Validation Evidence

Retained validation artefacts are stored under:

```text
evidence/A1
evidence/A2
evidence/A3
evidence/A4
evidence/A5
evidence/A6
evidence/A7
evidence/A8
evidence/A9
evidence/A10
```

The evidence pack files are:

- `evidence-pack-manifest.json`, which summarises the latest retained evidence.
- `evidence-pack.tgz`, which packages the retained evidence artefacts.

The A1-A10 evidence set corresponds to the acceptance criteria for mandate refusal, policy denial, authorised annotation, prompt-injection control, admission rejection, hostile egress tests, audit tamper detection, latency measurement, audit-outage denial, and privacy scanning.

## Privacy and Safety Constraints

- Synthetic fixtures only.
- No real personal data.
- No live operational deployment.
- No live recruitment decisions.
- The audit chain excludes direct identifiers, raw CV text, raw email addresses, phone numbers, salts, and human free-text reasons.
- Human free-text reasons are represented by fixture-store references.
- Unavailability never grants permission.
- The ability container communicates with `gear-pep` over loopback; trusted GEAR components mediate policy, inference, store access, and effects.

## Repository Layout

```text
api/          Kubernetes resource types
cmd/          Service entry points
internal/     Core implementation packages
policy/       OPA/Rego policy bundle
inference/    Synthetic extraction service
console/      HR demonstration UI
deploy/       Kubernetes manifests
harness/      Experiments and evidence tooling
evidence/     Retained validation artefacts
contracts/    Interface and invariant documents
```
