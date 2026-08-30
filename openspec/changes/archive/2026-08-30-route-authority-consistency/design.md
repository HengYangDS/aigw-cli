## Context

See [proposal.md](proposal.md). The persisted v3 configuration already
requires every Profile to declare one admitted client and one model. `test` and
`verify` nevertheless accept both `--profile` and `--for`, making callers
provide two values for one selection decision.

## Goals / Non-Goals

**Goals:**

- Give profile-based and client-route-based execution one authority each.
- Reject redundant selection before network or credential access.
- Keep current one-time v2 migration at the storage boundary.

**Non-Goals:**

- Remove `--for` from commands that intentionally operate on a selected client
  Route.
- Change Profile creation, Route persistence, credential storage, or Adapter
  behavior.
- Add aliases or compatibility shims for the rejected combination.

## Decisions

### Treat the selectors as alternatives

`--profile <profile>` derives the client from the Profile. `--for <client>`
operates on that client's selected Route. Supplying both fails before loading a
credential or making a request.

Alternative rejected: accept matching values and reject only conflicts. That
retains two inputs for one decision, increases the test matrix, and makes
automation depend on a relation that adds no capability.

### Keep migration isolated from current semantics

The v2 reader remains the only legacy boundary. Runtime configuration and
public commands continue to operate exclusively on the v3 client Route map.

Alternative rejected: remove the v2 reader in this atom. Existing installed
configurations still need the already-specified deterministic one-time
migration; removing it is a separate product compatibility decision.

## Risks / Trade-offs

- **Risk:** Existing automation supplies both selectors. **Mitigation:** fail
  with one precise correction: keep either `--profile` or `--for`.
- **Risk:** Broad deletion of `--for` could remove useful client-route
  operations. **Mitigation:** change only the redundant combined form.
