# progressive-team-onboarding Specification

## Purpose

Define progressive team setup as the independent import of reviewed provider
capability, followed by explicit Account connection and projection only to
clients that are present on the current host.

## Requirements

### Requirement: Team setup imports capability before activation

`aigw setup --from` SHALL import a credential-free Account and Profile
catalogue independently from Account connection, Route activation, client
installation, and external endpoint availability. No particular Account,
client, or external Responses service SHALL be mandatory for import. Setup
SHALL connect only the explicitly selected Account or Accounts whose Tokens are
already available through the selected credential backend.

#### Scenario: No Token or client is present

- **WHEN** a user imports a valid team manifest on a host with no admitted
  client and no Account Token
- **THEN** setup SHALL save the catalogue without prompting for every Account
- **AND** SHALL report that zero Accounts are connected
- **AND** SHALL show the smallest account-scoped connection command

#### Scenario: One Account is connected

- **WHEN** exactly one manifest Account is explicitly selected or has a locally
  available Token
- **THEN** setup SHALL select compatible profiles for that Account
- **AND** other manifest Accounts SHALL remain available but unconnected
- **AND** their missing Tokens SHALL NOT block setup

#### Scenario: Admitted clients are absent

- **WHEN** setup connects an Account before Claude Code or Codex is installed
- **THEN** setup SHALL preserve the connected Account and selected Routes
- **AND** SHALL identify `aigw sync` as the activation action after a client is
  installed

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
Every incomplete state SHALL expose one smallest safe next action. When no
Account is connected, that action SHALL identify one Account to connect rather
than imply that every catalogue Account is mandatory. After an Account Token or
client becomes available, setup guidance SHALL direct the operator to
`aigw sync`; `aigw check` SHALL remain verification of already enabled client
Routes, not an activation command. The setup command SHALL accept `--json` for
manifest-based setup and SHALL derive both representations from one semantic
result without exposing credential material.

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
- **THEN** setup SHALL name the environment variable for one Account as an
  example activation choice
- **AND** SHALL direct the operator to run `aigw sync` after setting it
- **AND** SHALL state that any one compatible Account is sufficient.

#### Scenario: Manifest setup is consumed by automation

- **WHEN** an operator runs manifest-based setup with `--json`
- **THEN** setup SHALL return the imported Account and Profile counts,
  connected Accounts, client states, and next safe action as machine-readable
  data
- **AND** SHALL NOT include an Account Token or credential value.

### Requirement: Deferred client activation remains explicit

AIGW SHALL allow team configuration import before clients or Tokens exist and
SHALL guide later client installation through projection, explicit native
authentication, and verification without making every provider mandatory.

#### Scenario: Codex is installed after setup

- **WHEN** an operator installs Codex after importing team configuration
- **THEN** aigw sync creates only the AIGW-owned projection
- **AND** status identifies explicit authentication as the next step when needed
- **AND** no unrelated provider Token is required
