## ADDED Requirements

### Requirement: Recovery advice follows the selected credential backend

When AIGW recommends providing or replacing an Account Token or a
provider-diagnostic credential, the recommendation SHALL be one that the
selected credential backend can perform. A writable backend SHALL name the AIGW
command that stores the credential. The read-only environment backend SHALL
name the exact environment variables to set and SHALL NOT recommend a command
that it refuses.

#### Scenario: The endpoint rejects an environment Token

- **WHEN** `aigw check` receives HTTP 401 for an Account whose Token comes from
  the environment backend
- **THEN** its human and JSON output SHALL name that Account's Token variable
- **AND** SHALL NOT recommend `aigw rotate`.

#### Scenario: Environment diagnostic credentials are absent

- **WHEN** `aigw account balance` finds no provider-diagnostic credential in the
  environment backend
- **THEN** it SHALL name both diagnostic variables for that Account
- **AND** SHALL NOT recommend `aigw account diagnostics enable`.

#### Scenario: A writable backend needs a credential

- **WHEN** the same failures occur with a writable backend
- **THEN** the recommendation SHALL remain `aigw rotate <account>` or
  `aigw account diagnostics enable <account>`.
