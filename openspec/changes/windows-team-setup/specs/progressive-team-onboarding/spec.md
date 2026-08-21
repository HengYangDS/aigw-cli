## ADDED Requirements

### Requirement: Team setup imports capability before activation

`aigw setup --from` SHALL import a credential-free Account and Profile
catalogue independently from Account connection, Route activation, client
installation, and external endpoint availability. No particular Account,
client, or external Responses service SHALL be mandatory for import.

#### Scenario: No Token or client is present

- **WHEN** a user imports a valid team manifest on a host with no admitted
  client and no Account Token
- **THEN** setup SHALL save the catalogue without prompting for every Account
- **AND** SHALL report that activation is deferred rather than claiming the
  workstation is ready.

#### Scenario: One Account is connected

- **WHEN** exactly one manifest Account has a locally available Token
- **THEN** setup SHALL select compatible profiles for that Account
- **AND** other manifest Accounts SHALL remain available but unconnected
- **AND** their missing Tokens SHALL NOT block setup.

### Requirement: Activation follows present capabilities

Setup SHALL validate and project only the intersection of selected connected
Accounts and clients discovered on the current host. A later synchronization
SHALL rediscover and adopt a newly installed admitted client without requiring
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

### Requirement: Onboarding state is explicit and actionable

Human and JSON results SHALL distinguish imported Accounts and Profiles,
connected Accounts, selected Routes, configured clients, and deferred work.
Every incomplete state SHALL expose one smallest safe next action.

#### Scenario: Selected Responses endpoint is unavailable

- **WHEN** an installed Codex route selects a Responses endpoint that is not
  reachable
- **THEN** diagnostics SHALL identify the configured endpoint as unavailable
- **AND** SHALL state that AIGW owns configuration rather than endpoint
  lifecycle
- **AND** SHALL offer checking that endpoint or selecting another Responses
  profile without inferring the endpoint implementation.
