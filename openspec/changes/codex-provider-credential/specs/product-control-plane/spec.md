## MODIFIED Requirements

### Requirement: Independently admitted native clients

Codex and Claude Code SHALL be the admitted native clients. Each adapter SHALL
own discovery, supported configuration or process planning, authentication,
rollback, verification, status, and uninstall of only its AIGW-owned state.
Credential retrieval for an admitted client SHALL resolve that client's active
Profile and selected Account through the single AIGW Token authority. Claude
Code's owned credential helper and an explicit Codex native-provider
authentication command SHALL invoke the installed AIGW executable by an
absolute, shell-safe path and SHALL NOT place a plaintext Token in client
configuration. AIGW-owned Claude invocations SHALL use Claude Code's
non-experimental compatibility mode so an ordinary admitted
Anthropic-compatible endpoint is not required to implement optional beta
negotiation. Adding a future client MUST NOT change provider policy or another
adapter.

#### Scenario: One admitted client is absent

- **WHEN** setup discovers only Codex or only Claude Code
- **THEN** AIGW SHALL configure only the present client
- **AND** it SHALL explicitly leave the absent client untouched

#### Scenario: Claude launches outside the installer shell

- **WHEN** Claude Code requests a credential from an enabled AIGW projection
- **THEN** `apiKeyHelper` SHALL invoke the exact installed AIGW executable
- **AND** credential retrieval SHALL not depend on the caller's PATH
- **AND** the projected settings SHALL contain no plaintext Token

#### Scenario: Codex authenticates an explicit native provider

- **WHEN** an enabled Codex Profile selects an explicit native provider identity
- **THEN** the projected authentication command SHALL invoke the exact installed
  AIGW executable for the Codex client
- **AND** it SHALL return only the Token of the Account selected by the active
  Codex Route
- **AND** the projected Codex configuration SHALL contain no plaintext Token

#### Scenario: Claude uses an Anthropic-compatible provider

- **WHEN** AIGW launches Claude Code for an admitted Claude Profile
- **THEN** the process SHALL disable optional experimental beta negotiation
- **AND** an ambient compatibility value SHALL NOT override the AIGW-owned value
- **AND** the setting SHALL remain process-local

#### Scenario: The installed executable path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the AIGW executable path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: Credential retrieval is not admitted

- **WHEN** a credential request names an unsupported client, a disabled adapter,
  an unresolved Route, or an Account without a Token
- **THEN** the request SHALL fail without writing credential bytes to standard
  output

#### Scenario: A future agent is admitted

- **WHEN** Hermes or another agent supporting third-party LLM APIs is proposed
- **THEN** admission SHALL require only that agent's adapter, declaration, and
  fixtures and SHALL NOT change provider policy, Proxy behavior, command roots,
  or an existing adapter

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW SHALL project the selected Profile once into that shared home
- **AND** SHALL NOT create a second Desktop-specific configuration authority
