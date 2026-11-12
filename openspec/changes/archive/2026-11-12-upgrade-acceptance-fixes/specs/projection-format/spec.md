# Spec Delta

## MODIFIED Requirements

### Requirement: Claude model preference and connection ownership are distinct

AIGW SHALL accept Claude Code's native model preference only when its sidecar
proves a model-only edit to an owned connection. Sync SHALL preserve it while
the selected model and credential scope are unchanged; otherwise it SHALL
reproject the selected model. External connection edits remain conflicts.
Inspection SHALL distinguish native preference from upstream model proof, and
compensation SHALL preserve observed bytes.

#### Scenario: A client rewrites equivalent JSON string escapes

- **WHEN** the client escapes slashes or Unicode without changing managed values
- **THEN** ownership verification accepts the unchanged values
- **AND** readiness remains valid and synchronization performs no writes to
  settings or ownership state merely to normalize JSON spelling
- **AND** a real projection change still compensates to the exact observed bytes.

#### Scenario: The user changes or removes the projected model

- **WHEN** the previous model reconstructs the recorded managed hash
- **THEN** synchronization of an unchanged connection performs no write and
  preserves the user's model preference, including absence
- **AND** preview reports an already converged client configuration file
  without writing any file
- **AND** withdrawal preserves the user's later model preference, including absence.

#### Scenario: AIGW changes only the credential helper executable

- **WHEN** the previous model reconstructs the recorded managed hash and the
  selected model and its provider and endpoint credential scope remain
  unchanged
- **THEN** AIGW projects the new helper without replacing the native model
- **AND** inspection still identifies the model as a native preference
- **AND** rollback restores the exact observed settings and
  sidecar bytes.

#### Scenario: AIGW changes the selected connection

- **WHEN** the selected model, its provider, or the endpoint changes
- **THEN** projection uses the selected model rather than carrying a native
  preference across a different credential scope.

#### Scenario: Read-only inspection sees a model-only preference

- **WHEN** the sidecar hash is reproduced by substituting only the previously
  selected model for the user's current top-level model
- **THEN** inspection accepts the AIGW-owned endpoint and credential helper
  without changing settings, sidecar bytes, or the user's model
- **AND** it identifies the model as a native preference rather than treating
  its spelling as a proven upstream wire model
- **AND** a changed selected model or credential scope reprojects the selected
  model.

#### Scenario: A model edit accompanies a connection or credential edit

- **WHEN** substituting only the previous model cannot reproduce the managed hash
- **THEN** synchronization rejects the conflict without adopting the changed fields.

#### Scenario: The first setup after an upgrade meets a model preference

- **WHEN** AIGW holds no earlier model for Claude Code and substituting the
  model of the new selection for the user's current top-level model, including
  its absence, reproduces the managed hash that an earlier release recorded
- **THEN** setup SHALL accept the edit as the user's model preference and
  project the connection
- **AND** it SHALL keep that preference only when the new selection keeps the
  provider and endpoint for which the earlier release projected the credential
  helper, and otherwise SHALL project the selected model
- **AND** any other change to the managed values SHALL stay a conflict.

### Requirement: An outside edit to managed client settings has one recovery action

When AIGW refuses because AIGW-managed settings of any client were changed
outside AIGW, every surface that reports the refusal, including errors, `check`
and `status`,
SHALL give the same next action: undo the outside edit, then run `aigw sync`,
or, before a configuration exists, the refused `aigw setup` again. AIGW SHALL
NOT advise a command that refuses on the same state, overwrite the edit, or
relax the guard.

#### Scenario: Retry budgets were edited outside AIGW

- **WHEN** the operator changed managed Codex retry budgets outside AIGW
- **THEN** `use --dry-run`, `client edit --dry-run`, `sync --dry-run`, `sync`,
  `check` and `status` SHALL report the same next action
- **AND** none of them SHALL advise `aigw check` or `aigw sync` alone
- **AND** client files and owned state SHALL stay unchanged.

#### Scenario: The outside edit is undone

- **WHEN** the operator restores AIGW's values and runs `aigw sync`
- **THEN** synchronization SHALL succeed and `check` SHALL report no drift.

#### Scenario: No configuration exists yet

- **WHEN** `aigw setup` refuses an outside edit in a home without `aigw.toml`
- **THEN** its next action SHALL be to undo the edit and run that `aigw setup`
  again, never `aigw sync`, which refuses without a configuration.
