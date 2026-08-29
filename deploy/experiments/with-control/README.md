# A6 With Control

The ability pod includes the PEP sidecar and an init container with UID-scoped egress controls.

Expected rule intent:

```text
iptables -A OUTPUT -o lo -j ACCEPT
iptables -A OUTPUT -m owner --uid-owner 1337 -j ACCEPT
iptables -A OUTPUT -j REJECT --reject-with icmp-admin-prohibited
```

Hostile ability-container egress must fail in this condition.

Run through:

```bash
make experiment ID=A6
```

The harness writes the rendered pod state, ability logs, init-container logs, NetworkPolicy state, Cilium status, and the pass/fail result into `evidence/A6/<timestamp>/`.
