## MODIFIED Requirements

### Requirement: Provider-neutral configuration

AIGW SHALL model Accounts, Profiles, Routes, protocol endpoints, and native
client model choices without provider identity hacks, named gateway products,
or deployment topology. Configuration manifests MUST remain credential-free
and SHALL describe available capability rather than requiring every Account to
be connected during import. Diagnostics SHALL require a credential only for an
Account selected by an active admitted-client Route.

#### Scenario: Import a multi-provider team catalogue

- **WHEN** an operator imports Accounts and Profiles for several providers
- **THEN** AIGW SHALL preserve all reviewed public capability
- **AND** SHALL allow zero or any subset of Account Tokens to be connected
- **AND** SHALL NOT require an unselected provider or absent client.

#### Scenario: Diagnose a partially connected team catalogue

- **WHEN** a reviewed catalogue contains multiple Accounts
- **AND** every Account selected by an active client Route has its Token
- **THEN** `aigw doctor` SHALL report the credential state as healthy
- **AND** SHALL NOT fail for an unselected Account whose Token is absent.

#### Scenario: Diagnose a selected Account without a Token

- **WHEN** an active client Route selects an Account whose Token is absent
- **THEN** `aigw doctor` SHALL report that Account as unhealthy
- **AND** SHALL provide the account-scoped rotation action.

#### Scenario: Add an ordinary provider

- **WHEN** an operator imports token-free Account and Profile data for a new
  endpoint
- **THEN** Codex or Claude Code SHALL select it without a provider-specific CLI,
  installer, projection branch, service manager, or core dependency.

#### Scenario: A Responses endpoint needs compatibility behavior

- **WHEN** an endpoint needs storage, replay, or other wire compatibility
- **THEN** AIGW SHALL NOT rename its provider identity or encode transport
  behavior in Account metadata.

#### Scenario: Reject implicit credential transport

- **WHEN** an imported manifest contains a token, password, authorization
  header, API key, or equivalent credential field
- **THEN** AIGW SHALL reject the manifest without changing local configuration.
