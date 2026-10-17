# Spec Delta

## ADDED Requirements

### Requirement: One-client checks do not observe unrelated clients

`aigw check` SHALL examine every enabled Client Binding when no client is
specified. `aigw check --for <client>` SHALL instead examine only that admitted,
enabled client. The scoped check SHALL not inspect another client's projection,
read another Account's credential metadata or Token, or contact another
endpoint. Human and JSON results SHALL report the checked scope; JSON
`enabled_clients` SHALL count, and `clients` SHALL list, only the requested
client.

#### Scenario: One client is healthy while another is not ready

- **GIVEN** two enabled clients with different Account Tokens and endpoints
- **WHEN** the operator checks one of them with `--for`
- **THEN** only that client's selected Route receives the requested endpoint or
  inference diagnostic
- **AND** the other client's missing Token or projection cannot block the
  scoped result.

#### Scenario: The requested client is unknown or disabled

- **WHEN** `--for` names an unknown or disabled client
- **THEN** check fails with a safe action before reading credential metadata or
  contacting any endpoint.

## MODIFIED Requirements

### Requirement: Readiness has one machine-readable projection

The readiness command SHALL accept `--json` and emit a stable JSON document without mutating configuration, credentials, client files, or network state beyond the existing read-only diagnostics. On macOS, the installed program's first read of a retained Keychain item MAY perform the one-time migration to AIGW ownership, which keeps the Token, service and Account unchanged.

#### Scenario: Configured routes are reported as structured facts

- **WHEN** `aigw check --json` runs with valid configuration and enabled client routes
- **THEN** it emits one JSON document containing each enabled route's client, selected route, account, endpoint readiness, adapter readiness, and overall result
- **AND** the command uses the same readiness evaluation as human-readable `aigw check`

#### Scenario: Optional catalogue entries do not block readiness

- **WHEN** the configuration contains unselected Accounts or Routes without Tokens
- **THEN** `aigw check --json` does not require their Tokens
- **AND** the result identifies only active routes as readiness requirements

#### Scenario: Missing active credentials remain actionable

- **WHEN** an enabled Account-Token route lacks its required Token
- **THEN** the command returns a non-zero exit status
- **AND** its JSON result identifies the missing Account and a safe next action without exposing Token material

#### Scenario: Client-native authentication is selected

- **WHEN** an enabled Route uses client-native authentication
- **THEN** check SHALL validate its local projection without reading AIGW Tokens
  or client-owned credentials
- **AND** its successful result SHALL remain local configuration evidence rather
  than authenticated endpoint or model evidence.
