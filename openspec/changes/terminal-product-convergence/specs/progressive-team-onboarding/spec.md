# Spec Delta

## MODIFIED Requirements

### Requirement: Onboarding state is explicit and actionable

Setup results SHALL distinguish imported capability, connected Accounts,
selected Routes, configured clients, and deferred work, and expose the smallest
safe next action. With environment credentials, setup SHALL list every
compatible variable as an equal choice and state that one is sufficient. After
a Token or client appears, continuation is `aigw sync`; `aigw check` only
verifies enabled Routes. Manifest setup MUST support `--json`, share one
semantic result, and never expose credentials.

#### Scenario: Guided setup completes before client installation

- **WHEN** guided setup connects an Account for a selected admitted client that
  is not yet installed
- **THEN** the wizard SHALL offer the admitted clients and require an explicit
  protocol choice when the selected client admits more than one
- **AND** setup SHALL preserve the Account, Token, Route, and client selection
  without claiming that native projection or inference is ready
- **AND** SHALL name client installation followed by `aigw sync`, not
  `aigw check`, as the activation continuation.

#### Scenario: Selected Responses endpoint is unavailable

- **WHEN** an installed Codex route selects a Responses endpoint that is not
  reachable
- **THEN** diagnostics SHALL identify the configured endpoint as unavailable
- **AND** SHALL state that AIGW owns configuration rather than endpoint
  lifecycle
- **AND** SHALL offer checking that endpoint or selecting another Responses
  route without inferring the endpoint implementation.

#### Scenario: No Token is available through a writable backend

- **WHEN** a valid team manifest is imported and no Account is connected
- **THEN** setup SHALL recommend `aigw rotate <account>`
- **AND** SHALL NOT imply that every catalogue Account Token is required.

#### Scenario: No Token is available through the environment backend

- **WHEN** a valid team manifest is imported with the read-only environment
  backend and no Account Token is available
- **THEN** setup SHALL enumerate the environment variable for every compatible
  Account as an alternative activation choice
- **AND** SHALL direct the operator to run `aigw sync` after setting one
- **AND** SHALL state that any one compatible Account is sufficient.

#### Scenario: Environment Account becomes available later

- **WHEN** a team catalogue was imported without a connected Account
- **AND** exactly one compatible Account Token later becomes available through
  the environment backend
- **THEN** `aigw sync` SHALL activate Routes compatible with that Account
- **AND** SHALL NOT require the previously recommended Account or any unrelated
  Account Token.

#### Scenario: Connected Account precedes client installation

- **WHEN** setup or Route selection connects an Account and enables a selected
  Client Binding without recording a native executable for its projection
- **THEN** the Account, Token, Route, and enabled intent SHALL remain distinct
  from the deferred native projection and unverified inference
- **AND** setup, selection, sync preview, status, check, and doctor SHALL name
  installation if needed followed by `aigw sync`, not immediate `aigw check`,
  as the activation continuation
- **AND** check SHALL NOT probe the endpoint before projection is available;
  doctor MAY pass local diagnostics without claiming client readiness.

#### Scenario: Selected Account Token disappears before client installation

- **WHEN** an enabled Client Binding has no native projection and its selected
  Account Token becomes unavailable
- **THEN** observational commands SHALL report the missing Token and deferred
  projection as distinct facts without reading the Token value
- **AND** the next action SHALL restore that selected Account's Token before
  suggesting synchronization or endpoint verification
- **AND** restoring the Token SHALL reveal installation if needed followed by
  `aigw sync` as the remaining activation step.

#### Scenario: Manifest setup is consumed by automation

- **WHEN** an operator runs manifest-based setup with `--json`
- **THEN** setup SHALL return the imported Account and Route counts,
  connected Accounts, client states, alternative activation choices, and next
  safe action as machine-readable data
- **AND** SHALL NOT include an Account Token or credential value.

#### Scenario: Setup imports without activation

- **WHEN** no Token or supported client is available
- **THEN** setup reports the catalogue as imported
- **AND** does not claim any endpoint, credential, or client is ready.

#### Scenario: Import-only setup has no actionable synchronization

- **WHEN** a valid team catalogue is imported with no compatible Token and no
  admitted client available
- **THEN** setup SHALL identify the choice of any one compatible Account Token
  as a prerequisite, not present immediate synchronization as useful work
- **AND** setup, status, check, and sync dry-run SHALL agree on that prerequisite
- **AND** sync dry-run SHALL select no Route or provider by recommendation alone.
