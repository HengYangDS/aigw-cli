## MODIFIED Requirements

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
