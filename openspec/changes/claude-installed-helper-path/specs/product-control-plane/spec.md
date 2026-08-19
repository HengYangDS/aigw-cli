## MODIFIED Requirements

### Requirement: Independently admitted native clients

Codex and Claude Code SHALL be the admitted native clients. Each adapter SHALL
own discovery, supported configuration or process planning, authentication,
rollback, verification, status, and uninstall of only its AIGW-owned state.
Claude Code's owned credential helper SHALL invoke the installed AIGW
executable by an absolute, shell-safe path. Adding a future client MUST NOT
change provider policy or another adapter.

#### Scenario: One admitted client is absent

- **WHEN** setup discovers only Codex or only Claude Code
- **THEN** AIGW SHALL configure only the present client
- **AND** it SHALL explicitly leave the absent client untouched

#### Scenario: Claude launches outside the installer shell

- **WHEN** Claude Code requests a credential from an enabled AIGW projection
- **THEN** `apiKeyHelper` SHALL invoke the exact installed AIGW executable
- **AND** credential retrieval SHALL not depend on the caller's PATH
- **AND** the projected settings SHALL contain no plaintext Token

#### Scenario: The installed executable path is invalid

- **WHEN** a Claude projection is prepared with a relative path or control
  character in the AIGW executable path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: A future agent is admitted

- **WHEN** Hermes or another agent supporting third-party LLM APIs is proposed
- **THEN** admission SHALL require only that agent's adapter, declaration, and
  fixtures and SHALL NOT change provider policy, Proxy behavior, command roots,
  or an existing adapter

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW writes one atomic marked projection
- **AND** does not invent a separate Desktop configuration authority.
