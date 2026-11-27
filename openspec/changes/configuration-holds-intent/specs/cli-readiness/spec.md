## MODIFIED Requirements

### Requirement: A depth that a client cannot run is skipped

A depth that a checked client cannot run, or a local check that does not
apply, SHALL be skipped with its reason and SHALL NOT count as a failure.
`aigw check` SHALL exit 1 when a checked client failed, or when no client is
named and none is enabled, and 0 otherwise.

#### Scenario: Named clients decide the exit status

- **WHEN** `aigw check hermes` runs with no client enabled and Hermes passes
- **THEN** the command SHALL exit 0
- **AND** `aigw check` without a client argument SHALL exit 1 at any depth.

#### Scenario: A one-time Token is supplied

- **WHEN** `aigw check <client> --model <provider/model> --token-stdin`
  supplies a one-time Token
- **THEN** `local` SHALL skip its stored-Token check and give that reason.

#### Scenario: A client authenticates natively

- **WHEN** a checked client selects its own `credential_command`
- **THEN** its `endpoint` and `model` depths SHALL be skipped with that reason
- **AND** check SHALL validate its local configuration without reading AIGW
  Tokens or client-owned credentials, and its success SHALL remain local
  configuration evidence.

#### Scenario: Claude Code keeps a proven native model preference

- **WHEN** its attributed sidecar proves that only the top-level model that
  AIGW wrote changed, while the endpoint, helper and managed credentials did
  not
- **THEN** the local check SHALL accept the connection and report the native
  model preference without rewriting it
- **AND** check SHALL skip the `model` depth, because AIGW does not choose the
  model that Claude Code sends, and SHALL name
  `aigw check claude --depth client` for native-model proof
- **AND** an endpoint, helper or managed-credential edit SHALL remain invalid.

#### Scenario: Claude Desktop is checked at the client depth

- **WHEN** `aigw check --depth client` checks Claude Desktop
- **THEN** its `client` depth SHALL be skipped, because Claude Desktop needs
  acceptance inside the app
- **AND** the command SHALL exit 0 when no checked client failed.
