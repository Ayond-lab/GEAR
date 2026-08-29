# A6 Negative Control

The same hostile ability workload runs without the UID-scoped egress init container.

Hostile ability-container egress must succeed in this condition. If it fails here too, A6 is invalid because the positive control did not isolate the claimed mechanism.

The harness creates the negative-control pod without the `gear.eu/ability` label so the mutating webhook does not inject the init container. It then adds the label after creation so the pod is still selected by the ability egress NetworkPolicy baseline.

Run through:

```bash
make experiment ID=A6
```
