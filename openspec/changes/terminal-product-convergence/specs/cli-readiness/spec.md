# Spec Delta

## ADDED Requirements

### Requirement: One-client checks do not observe unrelated clients

`aigw check` SHALL examine every enabled Client Binding when no client is
specified. `aigw check --for <client>` SHALL instead examine only that admitted,
enabled client. The scoped check SHALL not inspect another client's projection,
read another Account's credential metadata or Token, or contact another
endpoint. Human and JSON results SHALL report the checked scope; JSON
`enabled_clients` and `clients` SHALL contain only the requested client.

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
