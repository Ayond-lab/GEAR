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
| with-control | `policy-dns-http` | no | `blocked` |
| with-control | `policy-podip-http` | no | `blocked` |
| with-control | `raw-tcp-policy` | no | `blocked` |
| without-control | `connector-write-http` | no | `blocked` |
| without-control | `dns-assisted-policy` | no | `blocked` |
| without-control | `fixture-dns-http` | no | `blocked` |
| without-control | `inference-dns-http` | no | `blocked` |
| without-control | `mtls-policy-443` | yes | `auth-failed` |
| without-control | `policy-dns-http` | no | `blocked` |
| without-control | `policy-podip-http` | yes | `reached` |
| without-control | `raw-tcp-policy` | yes | `reached` |

## Verdict

FAIL

## Falsification Condition

A6 is falsified if any with-control hostile egress reaches a non-loopback destination, or if the negative-control egress cannot reach the same destinations after the init container is removed.

## Failures

- without-control attack policy-dns-http did not reach http://a6-gear-policy.gear-system.svc.cluster.local:8080/policy: Get "http://a6-gear-policy.gear-system.svc.cluster.local:8080/policy": context deadline exceeded
- without-control attack inference-dns-http did not reach http://a6-gear-inference.gear-system.svc.cluster.local:8080/inference: Get "http://a6-gear-inference.gear-system.svc.cluster.local:8080/inference": context deadline exceeded
- without-control attack fixture-dns-http did not reach http://a6-gear-fixture-store.gear-system.svc.cluster.local:8080/fixture: Get "http://a6-gear-fixture-store.gear-system.svc.cluster.local:8080/fixture": dial tcp: lookup a6-gear-fixture-store.gear-system.svc.cluster.local: i/o timeout
- without-control attack connector-write-http did not reach http://a6-gear-fixture-store.gear-system.svc.cluster.local:8080/connector/candidate-record: Get "http://a6-gear-fixture-store.gear-system.svc.cluster.local:8080/connector/candidate-record": dial tcp: lookup a6-gear-fixture-store.gear-system.svc.cluster.local: i/o timeout
- without-control attack dns-assisted-policy did not reach a6-gear-policy.gear-system.svc.cluster.local: lookup a6-gear-policy.gear-system.svc.cluster.local on 10.43.0.10:53: dial udp 10.43.0.10:53: i/o timeout
