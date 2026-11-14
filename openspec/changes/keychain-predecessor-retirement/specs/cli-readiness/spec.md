## MODIFIED Requirements

### Requirement: Check reports one machine-readable document

`aigw check --json` SHALL write one JSON document from the same evaluation as
its human output, without changing configuration, credentials, client files or
network state beyond the requests of its depths.

#### Scenario: Each checked client is reported as structured facts

- **WHEN** `aigw check --json` runs with valid configuration and enabled
  clients
- **THEN** it SHALL write one document that gives, for each checked client, its
  `provider/model`, its provider, the depth it reached, each skipped depth with
  its reason and every local check, and the overall result
- **AND** human `aigw check` SHALL report the same evaluation.

#### Scenario: Optional entries do not block readiness

- **WHEN** the configuration holds providers or models that no checked client
  uses and that have no Token
- **THEN** `aigw check --json` SHALL NOT require their Tokens
- **AND** the result SHALL treat only the checked clients' models as readiness
  requirements.

#### Scenario: Missing active credentials remain actionable

- **WHEN** a checked client's provider lacks its required Token
- **THEN** the command SHALL exit 1
- **AND** its JSON result SHALL name the provider and one Fix without exposing
  Token material.
