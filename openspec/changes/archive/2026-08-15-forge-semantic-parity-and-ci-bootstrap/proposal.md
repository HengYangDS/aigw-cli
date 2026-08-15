## Why

A deterministic provider-identity replay may collapse semantically duplicate Git commits. The Forge parity check incorrectly treats raw historical commit multiplicity as product meaning, while GitLab Linux jobs inherit curl's HTTP/2 transport instability during the exact-version mise bootstrap.

## What Changes

- Define cross-Forge source parity by the canonical tip tree, while commit provenance remains independently signed and verified on each Forge.
- Collapse duplicate projected parents created by provider identity normalization.
- Keep the CUE pipeline as the sole CI authority and add bounded HTTP/1.1 retry semantics to the exact-version mise bootstrap.

## Impact

The change removes a false parity failure and makes GitLab Linux bootstrap resilient without coupling either Forge, introducing a second pipeline owner, or weakening signed-history verification.
