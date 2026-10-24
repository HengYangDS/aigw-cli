## MODIFIED Requirements

### Requirement: Model discovery reports catalogue evidence only

`aigw catalog` and `aigw models` SHALL share one Account catalogue observation
path. Route comparison SHALL distinguish listed, not listed, and an unobserved
catalogue with its reason. These commands MUST NOT describe catalogue membership
as inference reachability or native-client readiness. An incomplete, oversized
or ambiguous HTTP response SHALL produce a failed catalogue observation, not
partial success.

#### Scenario: A configured model appears in the catalogue

- **WHEN** an authenticated catalogue response lists the configured model ID
- **THEN** `models` reports that ID as listed
- **AND** performs no inference or client invocation.

#### Scenario: A model is absent from a successfully observed catalogue

- **WHEN** a complete catalogue does not list the configured model ID
- **THEN** `models` reports not listed rather than unavailable.

#### Scenario: Catalogue observation fails

- **WHEN** credentials cannot be observed, the endpoint is absent, the request
  fails, or the response is incomplete or oversized
- **THEN** the affected Account has no established model membership
- **AND** its observation reason remains visible without blocking other Accounts.

#### Scenario: A catalogue response is ambiguous

- **WHEN** a successful HTTP response has a null model list, repeats that list,
  or repeats an identifier member within one item
- **THEN** the affected Account has no established model membership, and
  `models` SHALL NOT report its Routes as not listed
- **AND** an empty list, the supported identifier shapes and other provider
  fields SHALL still produce a successful observation.
