## Context

See `proposal.md` for motivation. Current ETHOS resolves official OpenSpec and
compiles the selected change into a transient three-field Commitment. Tracked
root and archived Commitment files therefore duplicate authority and preserve
obsolete lifecycle state.

## Goals / Non-Goals

**Goals:**

- Retain one tracked change-intent authority.
- Remove obsolete tracked Commitment carriers in one auditable change.
- Preserve existing immutable evidence without adding a migration registry,
  compatibility adapter, or replacement carrier.

**Non-Goals:**

- Changing AIGW behavior, provider adapters, client projections, or releases.
- Deleting host-local ETHOS Attestations or Git history.
- Introducing repository-specific ETHOS lifecycle logic.

## Decisions

### Use official OpenSpec as the tracked authority

OpenSpec already owns proposals, deltas, designs, tasks, and archives. ETHOS
consumes that source and compiles its minimal Commitment in memory. A second
tracked form would require another currentness rule without adding capability.

Alternative considered: retain historical Commitment files for readability.
Rejected because OpenSpec archives and Git already provide history while the
extra files preserve obsolete machine state.

### Delete carriers without a compatibility phase

All tracked Commitment files are removed together after strict OpenSpec
validation. No manifest or tombstone is added: Git records the deletion, and
the accepted ETHOS runtime already supports transient compilation.

Alternative considered: migrate incrementally behind a compatibility reader.
Rejected because no repository consumer remains and the adapter would prolong
parallel semantics.

## Risks / Trade-offs

- Existing tooling could still read a tracked Commitment path -> Validate with
  the current accepted ETHOS runtime and repository gates before landing.
- Historical inspection loses a convenient duplicate file -> Use official
  OpenSpec archives, Git history, and Attestations, which already own those
  facts.

## Migration Plan

1. Validate this OpenSpec change strictly and confirm ETHOS selects it.
2. Remove every tracked root and per-change Commitment file.
3. Run focused governance checks, then the affected repository gate once.
4. Land and archive through public lifecycle commands.

Rollback is a Git revert of the accepted change; no product data or installed
projection is transformed.
