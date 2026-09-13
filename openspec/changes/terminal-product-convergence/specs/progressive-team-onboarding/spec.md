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

### Requirement: Setup has one explicit commit boundary

Setup SHALL treat configuration, credential slots, backend selection, client
projections, checkpoints, locks, and temporary files as one owned transaction
until configuration persistence and every participating client projection
succeed. A configuration-file write is an intermediate effect, not completion
of the whole setup transaction. Failure before full completion SHALL apply
guarded compensation to that attempt's unchanged postimages. Recovery failure
SHALL retain the original cause and exact unresolved resources; concurrent
user changes SHALL be preserved. Presentation is outside this transaction.

#### Scenario: Setup fails before commit

- **WHEN** guided or manifest setup fails before configuration and all selected
  client projections complete, including after the configuration file is written
- **THEN** compensation restores unchanged owned preimages and removes that
  attempt's new resources wherever recovery succeeds
- **AND** any failed recovery or concurrent postimage conflict remains explicit
  without overwriting newer user state or claiming complete restoration.

#### Scenario: Setup output fails after commit

- **WHEN** setup commits configuration, credentials, backend selection, and
  client projections but result rendering fails
- **THEN** the committed product state remains available
- **AND** the command reports only the output failure rather than claiming that
  setup was rolled back.

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

- **WHEN** guided setup connects an Account while neither Claude Code nor Codex
  is installed
- **THEN** setup SHALL identify `aigw sync` as the next action after installing
  a supported client
- **AND** SHALL NOT identify `aigw check` as the activation action.

#### Scenario: Selected Responses endpoint is unavailable

- **WHEN** an installed Codex route selects a Responses endpoint that is not
  reachable
- **THEN** diagnostics SHALL identify the configured endpoint as unavailable
- **AND** SHALL state that AIGW owns configuration rather than endpoint
  lifecycle
- **AND** SHALL offer checking that endpoint or selecting another Responses
  profile without inferring the endpoint implementation.

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

- **WHEN** setup connects an Account and no admitted client is installed
- **THEN** its human and machine-readable results SHALL identify `aigw sync` as
  the next action after client installation
- **AND** SHALL NOT identify an observational command as the activation action.

#### Scenario: Manifest setup is consumed by automation

- **WHEN** an operator runs manifest-based setup with `--json`
- **THEN** setup SHALL return the imported Account and Profile counts,
  connected Accounts, client states, alternative activation choices, and next
  safe action as machine-readable data
- **AND** SHALL NOT include an Account Token or credential value.

#### Scenario: Setup imports without activation

- **WHEN** no Token or supported client is available
- **THEN** setup reports the catalogue as imported
- **AND** does not claim any endpoint, credential, or client is ready.

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
