# Codex Forwarding Maintenance

## Why

Published AIGW 0.3.1 cannot select a forwarding destination for Codex without
replacing its Account upstream. That couples a transport repair to credential
identity and other clients. The operator approved an independent maintenance
release based on the exact published 0.3.1 source, rather than publishing the
unfinished 0.4.0 product candidate.

## What Changes

- Add an explicit Codex-only forwarding destination to the existing Client
  Binding, resolver, selection and guarded native projection.
- Preview the destination without reading credential values or writing files;
  restore direct routing through the same selection and compensation owners.
- Keep the main schema-six configuration readable by the original installed
  credential reader. Include the owned destination component in snapshots,
  backups, verification and rollback.
- Deliver one signed, independently qualified maintenance release through the
  original Homebrew installation owner.

## Capabilities

### Modified Capabilities

- `route-client-selection`: Codex may explicitly select a forwarding transport
  without changing its Account or native credential identity.
- `projection-format`: the effective Codex destination participates in guarded
  projection and complete recovery without widening ownership.
- `release-distribution`: maintenance chronology follows source-lineage tags
  while all local product tags constrain version allocation and precedence.

## Impact

Start from signed `v0.3.1` source
`d288eb5fcc68215fad125e825eae99296819e70c`. The maintenance product version is
`0.3.3`; the existing local signed `v0.3.2` remains unchanged. Reuse the existing
locked toolchain, native lifecycle, signing, notarization and distribution
owners. Preserve the original credential backend and reader, Account records,
Hermes and other clients, provider identity, models, effort, and conversations.
Do not merge the general registry, catalogue, benchmark, credential-entrypoint,
or other unfinished 0.4.0 changes into this release.
