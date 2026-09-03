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

### Requirement: Setup has one explicit commit boundary

Setup SHALL treat configuration, credential slots, backend selection, client
projections, checkpoints, locks, and temporary files as one owned transaction
until the canonical configuration commit succeeds. Failure before that boundary
SHALL compensate only unchanged state written by the current transaction. A
presentation or output failure after that boundary SHALL report the error
without undoing committed product state.

#### Scenario: Setup fails before commit

- **WHEN** either guided or manifest setup fails before configuration commit
- **THEN** every AIGW-owned preexisting state remains byte-identical and every
  new owned artifact from that attempt is absent
- **AND** a concurrently changed credential or backend selection remains
  unchanged and the compensation conflict is reported.

#### Scenario: Setup output fails after commit

- **WHEN** setup commits configuration, credentials, backend selection, and
  client projections but result rendering fails
- **THEN** the committed product state remains available
- **AND** the command reports only the output failure rather than claiming that
  setup was rolled back.
