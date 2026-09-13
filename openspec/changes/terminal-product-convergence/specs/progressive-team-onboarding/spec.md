## ADDED Requirements

### Requirement: Deferred activation has one resumable path

Importing a reviewed team manifest SHALL establish available capability without
requiring every Account Token or supported client. `aigw sync` SHALL later
converge newly available credentials and clients without requiring setup to be
repeated or a hidden bulk-selection step. Imported recommendations SHALL remain
distinct from selected per-client Routes. Setup and synchronization SHALL fill
only unselected Routes: prefer a usable recommendation, then a compatible
Profile with the recommended model, then the first usable Profile in stable
identifier order. Existing selections SHALL remain unchanged even when their
credentials are unavailable.

#### Scenario: Any one Account is available

- **WHEN** a team manifest declares several Accounts and exactly one referenced
  Account Token is available
- **THEN** setup completes with that Account's compatible capability
- **AND** missing Tokens remain explicit deferred actions rather than errors for
  unrelated active Routes.

#### Scenario: A Token becomes available later

- **WHEN** an Account Token becomes available after manifest import
- **THEN** synchronization activates its compatible Profiles for unselected
  clients without requiring the originally recommended Account
- **AND** existing independent Routes are preserved.

#### Scenario: Recommendation survives deferred setup

- **WHEN** setup imports a recommendation while no required Token is available
- **THEN** the recommendation is persisted without becoming a selected Route
- **AND** a later usable recommendation wins over lexical Profile order.

#### Scenario: Explicit selection is temporarily unavailable

- **WHEN** a client has a selected Profile whose Token is absent and another
  compatible Account becomes connected
- **THEN** setup and synchronization preserve that selected Profile
- **AND** they do not replace its selection with the recommendation or another
  available Account.

#### Scenario: Recommended Profile is renamed or removed

- **WHEN** a Profile is renamed or an unselected Profile is removed
- **THEN** its recommendation reference is renamed or removed in the same
  configuration transaction
- **AND** other recommendations and selected Routes remain unchanged.

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

## MODIFIED Requirements

### Requirement: Activation follows present capabilities

Setup SHALL validate and project only discovered clients whose selected
Profiles have usable declared authentication: a required Account Token or
client-native ownership. A later synchronization SHALL rediscover and adopt a
newly installed admitted client without requiring
manifest re-import.

#### Scenario: Only Claude Code is installed

- **WHEN** a connected Account has both Anthropic and Responses profiles but
  only Claude Code is discovered
- **THEN** setup SHALL validate and configure only the Claude route
- **AND** an unavailable loopback Responses endpoint SHALL NOT block setup.

#### Scenario: Client is installed later

- **WHEN** a manifest was imported before Claude Code or Codex was installed
- **AND** the user later runs synchronization after installing that client
- **THEN** AIGW SHALL discover the client and converge its owned projection
- **AND** SHALL preserve unrelated client and conversation state.
