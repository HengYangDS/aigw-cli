## MODIFIED Requirements

### Requirement: Claude model preference and connection ownership are distinct

AIGW SHALL accept Claude Code's native model preference only when its sidecar
proves a model-only edit to an owned connection. Sync SHALL preserve it while
Route model and credential scope are unchanged; otherwise it SHALL reproject
the Route model. External connection edits remain conflicts. Inspection SHALL
distinguish native preference from upstream model proof, and compensation
SHALL preserve observed bytes.

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
- **AND** preview reports an already-converged projection without writing any file
- **AND** withdrawal preserves the user's later model preference, including absence.

#### Scenario: AIGW changes only the credential helper executable

- **WHEN** the previous model reconstructs the recorded managed hash and the
  Route model and Account/endpoint credential scope remain unchanged
- **THEN** AIGW projects the new helper without replacing the native model
- **AND** inspection still identifies the model as a native preference
- **AND** rollback restores the exact observed settings and
  sidecar bytes.

#### Scenario: AIGW changes the selected connection

- **WHEN** the Route model, Account, or endpoint changes
- **THEN** projection uses the selected Route's model rather than carrying
  a native preference across a different credential scope.

#### Scenario: Read-only inspection sees a model-only preference

- **WHEN** the sidecar hash is reproduced by substituting only the prior
  Route's model for the user's current top-level model
- **THEN** inspection accepts the AIGW-owned endpoint and credential helper
  without changing settings, sidecar bytes, or the user's model
- **AND** it identifies the model as a native preference rather than treating
  its spelling as a proven upstream wire model
- **AND** a changed Route model or credential scope reprojects the Route model.

#### Scenario: A model edit accompanies a connection or credential edit

- **WHEN** substituting only the previous model cannot reproduce the managed hash
- **THEN** synchronization rejects the conflict without adopting the changed fields.
