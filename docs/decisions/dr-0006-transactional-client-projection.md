# DR-0006: Project Client State as One Guarded Transaction

- Status: accepted
- Date: 2026-08-07

## Context

One configuration change may affect several admitted Codex homes and client
artifacts. Sequential best-effort writes can expose partial desired state, and
an unconditional rollback can overwrite a newer writer.

## Decision

AIGW prepares every selected target before committing any target. Each write
captures exact pre-state and post-state. If a commit fails, compensation runs
in reverse order only while the current bytes still match this transaction's
postimage. Drift fails closed instead of overwriting a later writer.

Dry-run returns the complete secret-free plan without reading credentials or
mutating client state. Missing clients and unowned keys are absent from the
transaction.

## Consequences

Partial projection is a failed outcome rather than a tolerated steady state.
Repair can reconcile all owned targets from the Account, Profile, and Route
SSOT without adopting foreign state.

## Revisit Trigger

Revisit if every admitted client projection moves behind one stronger atomic
storage primitive with equivalent preimage and concurrent-writer protection.
