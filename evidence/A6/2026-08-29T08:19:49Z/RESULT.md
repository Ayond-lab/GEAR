# A6 Hostile Egress Experiment

## Hypothesis

Ability-container non-loopback egress fails when the UID-scoped init-container control is present, and the same egress succeeds when that init container is removed.

## Method

The harness deployed three synthetic GEAR destination services, ran eight hostile egress paths from a labelled ability pod, then ran the same paths from an equivalent pod without the UID egress init container. The negative-control pod received the `gear.eu/ability` label after creation so the NetworkPolicy baseline still applied.

## Result

| Condition | Attack | Reached target | Outcome |
|---|---|---:|---|
| with-control | `connector-write-http` | no | `blocked` |
| with-control | `dns-assisted-policy` | no | `blocked` |
| with-control | `fixture-dns-http` | no | `blocked` |
| with-control | `inference-dns-http` | no | `blocked` |
| with-control | `mtls-policy-443` | no | `blocked` |
| with-control | `policy-clusterip-http` | no | `blocked` |
| with-control | `policy-dns-http` | no | `blocked` |
| with-control | `raw-tcp-policy` | no | `blocked` |
| without-control | `connector-write-http` | no | `blocked` |
| without-control | `dns-assisted-policy` | no | `blocked` |
| without-control | `fixture-dns-http` | no | `blocked` |
| without-control | `inference-dns-http` | no | `blocked` |
| without-control | `mtls-policy-443` | no | `blocked` |
| without-control | `policy-clusterip-http` | no | `blocked` |
| without-control | `policy-dns-http` | no | `blocked` |
| without-control | `raw-tcp-policy` | no | `blocked` |

## Verdict

FAIL

## Falsification Condition

A6 is falsified if any with-control hostile egress reaches a non-loopback destination, or if the negative-control egress cannot reach the same destinations after the init container is removed.

## Failures

- without-control attack policy-dns-http did not reach http://a6-gear-policy.gear-system.svc.cluster.local:8080/policy: Get "http://a6-gear-policy.gear-system.svc.cluster.local:8080/policy": context deadline exceeded
- without-control attack policy-clusterip-http did not reach http://10.43.62.235:8080/policy: Get "http://10.43.62.235:8080/policy": context deadline exceeded
- without-control attack inference-dns-http did not reach http://a6-gear-inference.gear-system.svc.cluster.local:8080/inference: Get "http://a6-gear-inference.gear-system.svc.cluster.local:8080/inference": dial tcp: lookup a6-gear-inference.gear-system.svc.cluster.local: i/o timeout
- without-control attack fixture-dns-http did not reach http://a6-gear-fixture-store.gear-system.svc.cluster.local:8080/fixture: Get "http://a6-gear-fixture-store.gear-system.svc.cluster.local:8080/fixture": context deadline exceeded
- without-control attack connector-write-http did not reach http://a6-gear-fixture-store.gear-system.svc.cluster.local:8080/connector/candidate-record: Get "http://a6-gear-fixture-store.gear-system.svc.cluster.local:8080/connector/candidate-record": dial tcp: lookup a6-gear-fixture-store.gear-system.svc.cluster.local: i/o timeout
- without-control attack dns-assisted-policy did not reach a6-gear-policy.gear-system.svc.cluster.local: lookup a6-gear-policy.gear-system.svc.cluster.local on 10.43.0.10:53: dial udp 10.43.0.10:53: i/o timeout
- without-control attack raw-tcp-policy did not reach 10.43.62.235:8080: dial tcp 10.43.62.235:8080: i/o timeout
- without-control attack mtls-policy-443 did not reach 10.43.62.235:443: dial tcp 10.43.62.235:443: i/o timeout
- without-control mTLS probe outcome "blocked", expected auth-failed
