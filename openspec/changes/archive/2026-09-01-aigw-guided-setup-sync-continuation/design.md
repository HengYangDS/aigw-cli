## Context

See [proposal.md](proposal.md). Guided setup already knows the authoritative
post-commit adapter state; the presentation layer currently ignores it when it
selects the next command.

## Goals / Non-Goals

**Goals:**

- Derive one accurate continuation from the committed configuration.
- Keep activation in the existing `aigw sync` command.

**Non-Goals:**

- Add another onboarding state model or compatibility path.
- Change discovery, credentials, synchronization, or client adapters.

## Decisions

Use the enabled adapter state already rendered by `renderSetupClients`. If at
least one client was configured, verification remains the next action. If none
was configured, setup explains deferred activation and points to `aigw sync`.
This avoids a parallel state owner and requires no new abstraction.

## Risks / Trade-offs

- **Risk:** Presentation diverges from actual state. **Mitigation:** Assert the
  user-visible continuation in the existing guided setup acceptance test.
