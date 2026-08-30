## Context

The setup domain already computes the catalogue, connected Accounts, selected
Routes, discovered clients, and next action before rendering human output. The
public command does not accept `--json`, so automation cannot consume the state
required by the existing progressive-onboarding specification.

## Goals / Non-Goals

**Goals:**

- Produce one immutable manifest-setup result and project it as human text or
  JSON.
- Keep credential values outside both projections.
- Preserve current setup mutation, validation, rollback, and client discovery
  semantics.

**Non-Goals:**

- Add a second setup workflow, credential backend, client Adapter, or external
  endpoint lifecycle owner.
- Make every command support JSON in this atom.
- Change direct, non-manifest setup behavior.

## Decisions

### Return a domain result before rendering

Manifest setup will construct a small result from values it already owns after
the transaction commits. Human rendering and JSON encoding consume that same
result. This avoids parsing display text or maintaining parallel behavior.

Alternative rejected: add a JSON-only execution path. It would duplicate
setup semantics and create drift risk.

### Keep the result declarative and secret-free

The result contains identifiers and state categories, never Token values. The
selected credential backend remains the sole credential authority.

Alternative rejected: expose backend details or credential probes. They are
diagnostic concerns and would broaden the setup contract unnecessarily.

## Risks / Trade-offs

- **Risk:** Human wording changes could accidentally alter machine semantics.
  **Mitigation:** Both renderers consume the same typed result and acceptance
  tests assert the JSON contract directly.
- **Risk:** A broad result type could become a second configuration model.
  **Mitigation:** Include only observable setup outcome fields; configuration
  remains authoritative in `internal/configuration`.
