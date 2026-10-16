# Disabled Binding Readiness

## Why

A disabled Client Binding can decide whether AIGW reports the enabled clients
as ready. During the 0.4.2 review on 2026-10-15, a reviewer reported that a
disabled Account's credential denial overrides a healthy enabled Account. With
Codex enabled on a healthy Account and Claude Code keeping a selected Route on
another Account whose binding is disabled, `aigw status` and `aigw doctor` read
that other Account's credential metadata. When the read fails, the disabled
client is reported as unavailable, status recommends `aigw doctor`, and doctor
reports problems.

The extra read was introduced on 2026-09-27 (`e311aa1e`), and 0.4.0, 0.4.1 and
0.4.2 contain it. The `cli-readiness` contract already says that no absent
client changes the result.

## What Changes

- Read-only readiness commands (`status`, `check` and `doctor`) observe
  credential metadata only for the Accounts that activation selects: the
  enabled clients' Accounts or, with no client enabled, the selected bindings
  that activation considers.
- A disabled binding outside that scope is reported as deferred because its
  client is not installed or enabled, and its Account is not read.
- A disabled binding's credential state can no longer change another client's
  state, the overall next action or the doctor result.

## Capabilities

`cli-readiness` gains the requirement "A disabled Client Binding cannot change
readiness".

## Impact

The owner is the client inspection in `internal/cli/readiness`, which `status`,
`check` and `doctor` share. Activation in `internal/activation` keeps choosing
which Accounts a read-only command observes, and the state classification in
`internal/readiness` is unchanged. Enabled clients, credential values,
configuration and native client files are unchanged. The fix is user-visible
and ships with a CHANGELOG entry in the next release.

Authority boundary: read-only commands; no credential value is read and no
prompt is opened. Breaking removals: none. Reuse: activation's existing
credential observations; no new store, cache or reader.

Non-goals: changing which Accounts activation observes; changing `verify`,
`sync` or Account rename; reporting a disabled binding's Token availability
before the binding is enabled.
