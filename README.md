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

## Local Checks

```bash
make controller-gen
make generate
make manifests
make test
make opa-test
```

`controller-gen` is installed into `bin/` by the Makefile. The first Milestone 1 wiring is present; strict generated CRDs and deepcopy output land when the API skeletons are converted to Kubernetes runtime types.

The API skeletons have now been converted to Kubernetes runtime types. `make generate` emits `api/v1/zz_generated.deepcopy.go`, and `make manifests` emits CRD schemas from the Go API definitions.

The first `gear-webhooks` validation slice is present in `internal/webhooks`: mandate creation/update validation resolves the referenced `Ability` and rejects mandates that widen the ability manifest. `make test-envtest` now runs the webhook validation tests.

`cmd/gear-webhooks` now starts a controller-runtime manager, registers the Mandate validating webhook at `/validate-gear-eu-v1-mandate`, and exposes health/readiness probes. `deploy/base` includes the webhook Deployment, Service, RBAC, namespace, and fail-closed `ValidatingWebhookConfiguration`.

Ability pods labelled `gear.eu/ability=<ability-name>` are now handled by a mutating webhook at `/mutate-v1-pod`. It injects the `gear-pep` sidecar, the UID-scoped egress init container, PEP-only credential volumes, disables pod-level service account token automounting, and pins ability/PEP UIDs to `1001` and `1337`.
