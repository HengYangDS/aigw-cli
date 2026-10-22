## MODIFIED Requirements

### Requirement: Ambiguous client protocols require an explicit choice

When a Route and Client Binding admit more than one endpoint protocol, AIGW SHALL
preserve an existing explicit protocol or require the operator to choose one.
It SHALL NOT infer a protocol from the Model name or silently prefer an
endpoint. Direct setup and Route selection SHALL expose that choice through
the same protocol names used by the team manifest.

#### Scenario: Hermes selects a multi-protocol Route

- **WHEN** Hermes selects a Route with multiple compatible protocols and no
  prior or recommended protocol applies
- **THEN** non-interactive selection names the required `--protocol` choice
- **AND** interactive selection offers each compatible protocol
- **AND** a failed or incompatible choice leaves every Client Binding unchanged.

#### Scenario: A selected protocol requires a new Account Token

- **WHEN** direct setup or interactive Route selection obtains a missing Token
  for a client with multiple compatible protocols
- **THEN** validation probes only the explicitly selected protocol and endpoint
- **AND** a failed probe does not store the Token or change the Client Binding.

#### Scenario: A non-interactive refusal names its retry

- **WHEN** non-interactive Route selection or direct setup refuses because more
  than one protocol is compatible
- **THEN** the recommended action, including JSON `next_action`, names the same
  operation with `--protocol` and the compatible protocol names
- **AND** Route selection names its exact retry,
  `aigw use --for <client> --protocol <a|b> <route>`.
