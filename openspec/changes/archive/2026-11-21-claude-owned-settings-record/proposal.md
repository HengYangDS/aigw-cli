## Why

The candidate of 0.9.0 failed its acceptance against the published 0.8.3 on 2026-11-21: after `aigw self rollback`,
the 0.8.3 program refused `aigw sync` with "AIGW's record settings.json.aigw-state.json cannot be read … unknown
field \"model_overrides\"". The change `client-model-windows` had added a field to the record of Claude Code's
settings and the map to the record's digest, and 0.8.3 reads that record strictly. `program-lifecycle` requires
that a client which stays enabled keeps working through upgrade, rollback and re-upgrade between a release and its
predecessor, so this is a defect of that change, and no tag was made.

## What Changes

- AIGW's record of Claude Code's settings returns to the fields and the digest that 0.8.3 reads.
- AIGW records a settings key that it newly owns, here `modelOverrides`, in a record of its own beside the first:
  what the key held before AIGW wrote it, and a digest of what AIGW wrote. Its reader keeps an entry that it does
  not know, so a later release can add one without breaking this one.
- Ownership of the map begins when AIGW first needs one. A map that the user wrote is taken over then, with its
  value kept for the withdrawal, and is left alone while AIGW needs none. The upgrade from 0.8.3 no longer refuses
  settings that already hold a map.
- The release lifecycle's journey writes a map, so the acceptance against a published predecessor exercises it.
- The changelog's entry for 0.9.0 loses its instruction to disable Claude Code before a rollback of the program.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `client-adapters`: "Claude Code maps Anthropic model IDs to the provider's" gains the scenarios of a user's own
  map; one added requirement, "A record that the previous release reads keeps its shape".

## Impact

`internal/adapter/claude` (the settings projection, its records, inspection and the verification copy), the release
journey's fixture and assertions under `tools/release`, `CHANGELOG.md` and the documents that describe the map.
Unreleased: no published program wrote the field that this removes.

- **Authority boundary:** the requirement that a rollback keeps an enabled client working is the accepted
  specification's; the acceptance owner of this repository (the AIGW and Proxy session) chose this form and told
  the owner on 2026-11-21.
- **Proof:** local tests prove the records and the transitions. Only the candidate's acceptance against the
  published 0.8.3 proves the rollback; it runs before the tag, as for every release.
- **Non-goals:** Claude Desktop's profile, which 0.8.3 reads tolerantly; a hosted job that uses a published
  predecessor by default; any change to what `aigw self rollback` does.
