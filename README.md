# GEAR Development Build

GEAR is a governed execution layer for consequential automation. This subtree contains the development implementation described by the technical specification: authority separation, sidecar enforcement, deterministic policy decisions, and a tamper-evident audit chain.

The initial scaffold is intentionally dependency-light. Milestone 0 establishes contracts, API shape, local invariants, conformance placeholders, and build targets. Later milestones can replace the local stand-ins with kubebuilder, Kubernetes clients, OPA bundle loading, bbolt persistence, and mTLS runtime wiring.

## Safety Boundary

- Synthetic data only.
- No live organisational deployment.
- No real personal data in fixtures, logs, audit entries, screenshots, or evidence.
- Default decision is deny.
- Policy input is exactly ten fields.
- Ability containers talk only to `gear-pep` on loopback.

## Milestones

1. Repository and contracts.
2. Cluster and admission baseline.
3. Policy and audit core.
4. PEP enforcement.
5. Mandate derivation.
6. CV demonstration path.
7. Console and evidence.

