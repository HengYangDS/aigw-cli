## Context

The setup domain already computes the catalogue, connected Accounts, selected
Routes, discovered clients, and next action before rendering human output. The
public command did not expose that state as JSON. More fundamentally, the
configuration stores `routes.default` plus `routes.overrides`, although a
Profile is client-scoped. A Codex Profile cannot be Claude's fallback and a
Claude Profile cannot be Codex's fallback. Treating either as a global default
creates an implicit third Route that no client necessarily uses.

## Goals / Non-Goals

**Goals:**

- Produce one immutable manifest-setup result and project it as human text or
  JSON.
- Keep credential values outside both projections.
- Preserve current setup mutation, validation, rollback, and client discovery
  semantics.
- Represent every active selection exactly once as `client -> Profile`.
- Make every runtime consumer resolve the same explicit client Route.

**Non-Goals:**

- Add a second setup workflow, credential backend, client Adapter, or external
  endpoint lifecycle owner.
- Make every command support JSON in this atom.
- Preserve the global default, inheritance, or an alias for either concept.
- Guess a Route for a newly admitted client.

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

### Use explicit client Routes as the sole selection authority

The local configuration stores one Route map keyed by admitted client. Each
selected Profile declares that same client and one model. Account is the
reusable provider boundary; Profile and Route remain client-specific. The old
`models.<client>` map is removed because it repeats `profile.client` and leaves
an unused cross-client degree of freedom.

`aigw use <profile>` derives its client from the Profile. A missing client is a
configuration error rather than an invitation to create an implicit default.
`--all` and route reset are removed because they either apply an incompatible
Profile to another client or restore a nonexistent inheritance relation.

Alternative rejected: keep `routes.default` only as a readiness fallback. That
would retain two authorities and preserve the original failure under a narrower
name.

Alternative rejected: define one default per model family outside Routes. That
would duplicate the client Route and make future client admission modify two
registries.

### Migrate the previous schema once, then write only the terminal form

The configuration schema advances. A previous client override maps directly to
the corresponding Route. A previous default is used only for its explicitly
declared client and only when that client lacks an override. Ambiguous generic
Profiles fail with an actionable selection error; migration never guesses.
Successful persistence emits only the new schema. The team manifest advances
and removes `recommended_default`; its per-client recommendations remain the
only onboarding selection input.

### Check enabled clients, not a synthetic service

Readiness resolves each enabled client's Route, verifies its credential and
Adapter, and probes each distinct `(account, endpoint, protocol)` target. A
shared Account may therefore be reused without duplicate probes, while Claude
and Codex remain independently verifiable. With no enabled client, readiness
checks configuration only and does not claim gateway health.

## Risks / Trade-offs

- **Risk:** Human wording changes could accidentally alter machine semantics.
  **Mitigation:** Both renderers consume the same typed result and acceptance
  tests assert the JSON contract directly.
- **Risk:** A broad result type could become a second configuration model.
  **Mitigation:** Include only observable setup outcome fields; configuration
  remains authoritative in `internal/configuration`.
- **Risk:** Existing schema-v2 installations contain only a global default.
  **Mitigation:** migrate only an unambiguous client-scoped Profile; otherwise
  stop with the exact `aigw use <profile>` actions required.
- **Risk:** Removing `--all` breaks memorized commands.
  **Mitigation:** the replacement is shorter and deterministic: invoke
  `aigw use <profile>` once for each intended client Profile.
