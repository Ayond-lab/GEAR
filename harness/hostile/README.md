# Hostile Experiments

`make experiment ID=A6` runs the A6 hostile egress experiment against the local `k3d` lab cluster.

The harness builds local lab images, deploys synthetic destination services, runs eight attack paths from the ability container, and writes retained evidence under:

```text
evidence/A6/<RFC3339 timestamp>/
```

Attack paths:

- Direct HTTP call to the policy service by DNS.
- Direct HTTP call to the policy service by Pod IP.
- Direct HTTP call to the inference service by DNS.
- Direct HTTP call to the fixture store by DNS.
- Direct connector-like write call.
- DNS-assisted service discovery.
- Raw TCP connection to the policy service by Pod IP.
- mTLS attempt to the policy service without a client certificate.

The with-control pod is labelled as an ability pod at creation time, so admission injects the PEP sidecar and UID-scoped egress init container. All eight non-loopback paths must fail from UID `1001`.

The negative-control pod has the same ability container, PEP sidecar, credential-volume shape, service-account-token setting, and NetworkPolicy selection, but it is created without the init container. The harness adds the ability label after creation. The same egress paths must then reach their synthetic targets, with the mTLS probe stopping at authentication rather than network reachability.
