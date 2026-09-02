## ADDED Requirements

### Requirement: Deferred activation has one resumable path

Importing a reviewed team manifest SHALL establish available capability without
requiring every Account Token or supported client. `aigw sync` SHALL later
converge newly available credentials and clients from the existing per-client
Routes without requiring setup to be repeated or a hidden bulk-selection step.

#### Scenario: Any one Account is available

- **WHEN** a team manifest declares several Accounts and exactly one referenced
  Account Token is available
- **THEN** setup completes with that Account's compatible capability
- **AND** missing Tokens remain explicit deferred actions rather than errors for
  unrelated active Routes.

#### Scenario: A Token becomes available later

- **WHEN** an Account Token becomes available after manifest import
- **THEN** synchronization can activate only compatible selected or recommended
  Routes for that Account
- **AND** existing independent Routes are preserved.

#### Scenario: A client is installed later

- **WHEN** a supported client is installed after setup
- **THEN** synchronization discovers and projects that client from its existing
  Route
- **AND** setup does not need to be repeated.

### Requirement: Setup reports progress rather than fictional completion

Setup SHALL distinguish imported capability, connected Accounts, selected
Routes, projected clients, and deferred actions in both human and JSON output.

#### Scenario: Setup imports without activation

- **WHEN** no Token or supported client is available
- **THEN** setup reports the catalogue as imported
- **AND** does not claim any endpoint, credential, or client is ready.
