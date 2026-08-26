# Hostile Experiments

This harness will run A6 attack paths against ability containers:

- Direct call to `gear-policy`.
- Direct call to `gear-inference`.
- Direct call to fixture store.
- Direct connector egress.
- DNS-assisted egress.
- Raw TCP egress.
- Attempted service-account token use.
- mTLS failure against `gear-policy`.

The negative control must remove the UID-scoped init container and prove hostile egress succeeds.

