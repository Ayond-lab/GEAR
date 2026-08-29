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
| without-control | `connector-write-http` | yes | `reached` |
| without-control | `dns-assisted-policy` | yes | `resolved` |
| without-control | `fixture-dns-http` | yes | `reached` |
| without-control | `inference-dns-http` | yes | `reached` |
| without-control | `mtls-policy-443` | yes | `auth-failed` |
| without-control | `policy-dns-http` | yes | `reached` |
| without-control | `policy-podip-http` | yes | `reached` |
| without-control | `raw-tcp-policy` | yes | `reached` |

## Verdict

PASS

## Falsification Condition

A6 is falsified if any with-control hostile egress reaches a non-loopback destination, or if the negative-control egress cannot reach the same destinations after the init container is removed.
