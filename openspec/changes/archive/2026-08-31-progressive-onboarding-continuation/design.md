## Context

The setup result already owns both human and JSON continuation output. The
defect is one branch in that existing semantic owner, not a missing lifecycle
or routing abstraction.

## Goals / Non-Goals

**Goals:**

- Keep the next action aligned with the command that can advance current state.
- Prove the result when an environment-backed Account is connected before any
  admitted client is installed.
- Make explicit Profile selection activate only its declared client when that
  client and credential are available.

**Non-Goals:**

- Change Account selection, credential storage, or projection formats.
- Add aliases, compatibility behavior, or another continuation model.

## Decisions

### Reuse the existing result owner

Change the existing no-selected-client branch from `status` to `sync`.
`sync` owns rediscovery and projection; `status` is observational and therefore
cannot satisfy this state transition.

### Add one end-to-end acceptance

Exercise manifest import with one environment Token and no discovered clients,
then assert the persisted Routes, connected Account, deferred explanation, and
exact next action. A separate helper or state machine would duplicate existing
behavior without adding an invariant.

### Reuse the synchronization domain

Move available-client configuration derivation from the recovery command into
the synchronization domain. `sync` and `repair` request all admitted clients;
`use` requests only the selected Profile's declared client. This keeps one
discovery and adapter-convergence implementation while preserving independent
client Routes.

## Risks / Trade-offs

- `sync` cannot configure a still-absent client. The deferred explanation
  already says to run it after installation, so the command remains truthful.
- `use` may now fail when projection cannot be applied; the existing
  transaction restores the Route and any newly acquired Token.
