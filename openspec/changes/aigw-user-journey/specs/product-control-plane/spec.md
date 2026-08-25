## MODIFIED Requirements

### Requirement: Reviewed team configuration is directly consumable

The repository SHALL publish exactly one token-free team manifest containing
the reviewed Accounts, Profiles, and recommended client Routes. It SHALL be
directly consumable by `aigw setup --from` without requiring a Token or an
installed client and SHALL NOT contain fictitious providers, credentials,
workstation paths, or a parallel example manifest. When exactly one Account is
connected, AIGW SHALL preserve the reviewed route's client and model intent
while selecting an equivalent Profile owned by that Account. A later
`aigw sync` SHALL discover supported clients and project only AIGW-owned
configuration without rebinding credentials.

#### Scenario: Team member imports reviewed settings

- **WHEN** a team member downloads the tracked manifest and runs `aigw setup --from`
- **THEN** AIGW SHALL import the reviewed DMXAPI, AIHubMix, and UCloud profiles
- **AND** SHALL request or reuse Tokens outside the manifest
- **AND** SHALL recommend GPT-5.6 Sol for Codex and Claude Fable 5 for Claude

#### Scenario: No Account is connected during import

- **WHEN** a user imports the team manifest without supplying a Token
- **THEN** every reviewed Account and Profile SHALL be retained
- **AND** no client installation or credential SHALL be required
- **AND** the next action SHALL explain how to connect one Account later.

#### Scenario: One Provider Account is connected

- **WHEN** a user imports the team manifest with exactly one available Account Token
- **THEN** setup SHALL succeed without Tokens for other Accounts
- **AND** each route SHALL select a compatible Profile owned by the connected Account
- **AND** selection SHALL preserve the reviewed model when that Account offers it
- **AND** a lexical fallback MAY be used only when no equivalent model exists.

#### Scenario: A supported client is installed later

- **WHEN** setup completed before Codex or Claude Code was installed
- **AND** the selected Account has a compatible route
- **THEN** `aigw sync` SHALL discover and project that client
- **AND** SHALL NOT require or replace any Token
- **AND** SHALL leave absent clients untouched.
