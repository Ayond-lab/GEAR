# Subsumption Contract

A mandate is valid only if every grant and boundary is equal to or narrower than the referenced ability manifest version.

The invariant is enforced at three points:

1. `gear-console` renders mandate options from the referenced manifest.
2. `gear-mandate` checks subsumption before signing.
3. `gear-webhooks` rejects invalid resources at Kubernetes admission time.

Admission rejection must hold even for direct `kubectl apply` by a cluster administrator.

For Milestone 0, `internal/subsume` checks the fields available in the dependency-light API skeleton: ability version, trigger source, connector scope, action class, disposition values, reversibility coverage, caps, and decimal threshold shape.

