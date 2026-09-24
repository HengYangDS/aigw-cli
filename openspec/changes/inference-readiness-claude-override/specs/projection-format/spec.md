## MODIFIED Requirements

### Requirement: Claude model preference and connection ownership are distinct

AIGW SHALL reconcile an explicit Claude Route when the recorded managed hash
proves that only the top-level model changed. The prior Route supplies the old
model; current user settings SHALL NOT establish connection ownership.
Read-only inspection SHALL accept that proven native model preference without
rewriting settings or sidecar state and SHALL distinguish it from the Route's
wire-model evidence. Endpoint, helper, and managed-credential changes SHALL
remain conflicts. AIGW SHALL preserve neighboring settings and byte-exact
compensation, and SHALL compare decoded strings rather than equivalent JSON
escapes.

#### Scenario: A client rewrites equivalent JSON string escapes

- **WHEN** the client escapes slashes or Unicode without changing managed values
- **THEN** ownership verification accepts the unchanged values
- **AND** readiness remains valid and synchronization performs no writes to
  settings or ownership state merely to normalize JSON spelling
- **AND** a real projection change still compensates to the exact observed bytes.

#### Scenario: The user changes or removes the projected model

- **WHEN** the previous model reconstructs the recorded managed hash
- **THEN** synchronization projects the selected Route's model
- **AND** preview reports Claude's pending projection without writing any file
- **AND** withdrawal preserves the user's later model preference, including absence.

#### Scenario: Read-only inspection sees a model-only preference

- **WHEN** the sidecar hash is reproduced by substituting only the prior
  Route's model for the user's current top-level model
- **THEN** inspection accepts the AIGW-owned endpoint and credential helper
  without changing settings, sidecar bytes, or the user's model
- **AND** it identifies the model as a native preference rather than treating
  its spelling as a proven upstream wire model
- **AND** an explicit Route selection remains the only operation that
  reprojects the Route's default model.

#### Scenario: A model edit accompanies a connection or credential edit

- **WHEN** substituting only the previous model cannot reproduce the managed hash
- **THEN** synchronization rejects the conflict without adopting the changed fields.
