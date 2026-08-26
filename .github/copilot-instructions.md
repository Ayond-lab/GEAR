# GEAR Development Instructions

GEAR is a governed execution layer for consequential automation. Keep every implementation decision aligned with these invariants:

- Synthetic data only for the TRL 4 build.
- Default outcome is deny.
- Policy input has exactly ten fields.
- Model output, prompt text, extracted free text, and ability narrative are not policy inputs.
- No personal data or salts enter the audit chain.
- The ability container only calls the PEP loopback API.
- Mandates may narrow ability manifests but never widen them.
- Audit decision entries are durable before decisions are returned.

