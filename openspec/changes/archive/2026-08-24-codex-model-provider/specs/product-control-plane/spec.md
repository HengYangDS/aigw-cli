## ADDED Requirements

### Requirement: Profile-scoped Codex native provider

A Profile SHALL remain the sole owner of its client, Account, model, and
optional client-native provider selection. A missing Codex provider selection
SHALL resolve to the canonical `aigw` provider. Provider selection SHALL NOT be
inferred from an Account, endpoint, proxy implementation, or client-private
state.

#### Scenario: Explicit Codex provider

- **WHEN** a Codex-scoped Profile declares a safe `model_provider`
- **THEN** its resolved Runtime carries that exact provider identity
- **AND** Codex receives one attributed native provider table using the
  Profile's Account endpoint

#### Scenario: Default Codex provider

- **WHEN** a Codex-scoped Profile omits `model_provider`
- **THEN** its Runtime resolves the canonical `aigw` provider
- **AND** the existing AIGW projection remains byte-compatible

#### Scenario: Provider ownership is narrow

- **WHEN** a non-Codex Profile or an unsafe provider identifier declares
  `model_provider`
- **THEN** configuration validation fails before persistence or projection

### Requirement: Provider-owned Codex authentication

The canonical `aigw` provider SHALL use generic Codex login and the AIGW model
catalogue. An explicit native provider SHALL instead use Codex command-backed
authentication with the absolute AIGW executable and SHALL NOT project a Token,
an environment-key alternative, `requires_openai_auth`, or the AIGW catalogue.

#### Scenario: Native provider projection

- **WHEN** AIGW synchronizes an explicit native provider
- **THEN** the provider table declares `wire_api = "responses"`
- **AND** its auth command invokes `aigw credential codex`
- **AND** generic Codex login is not requested

#### Scenario: Return to the default provider

- **WHEN** a Profile changes from an explicit provider to the default provider
- **THEN** the old attributed provider table is removed transactionally
- **AND** generic Codex authentication is rebound
