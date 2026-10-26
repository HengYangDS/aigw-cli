# Proposal

## Why

Codex can delegate request recovery to an explicitly selected endpoint through
native retry budgets, but AIGW cannot declare or preserve those fields. Its
strict provider reader also presents managed-schema conflicts as TOML syntax
errors, hiding the actual recovery boundary.

## What Changes

- Add optional connection-scoped request and stream retry budgets to a Client
  Binding and expose them through explicit `use` options and configuration.
- Project only declared budgets into Codex's native provider table; retain
  undeclared native defaults, unrelated bindings and all existing ownership guards.
- Reuse native persistence, guarded synchronization, compensation and rollback.
- Repair the diagnosed source-acceptance owners: native Mise startup isolation,
  GoReleaser existing-peer metadata selection and both mirror refusal fixtures.
- Preserve semantic provider-conflict diagnostics and reject unsupported client
  declarations before credential access or mutation.
- Reject incompatible program rollback using the retained reader's actual
  configuration capability, without discarding policy or requiring manual edits.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `route-client-selection`: explicit connection retry declaration and its
  projection, persistence, conflict and recovery boundaries.

## Impact

Configuration, Codex projection, selection, diagnostics, rollback admission,
their focused tests and command documentation change. No dependencies are added.
Codex implements only this repair; Claude retains publication and installation.
AIGW does not retry requests, manage external endpoints, guarantee exactly-once
execution, or change sessions, credentials or running services.
