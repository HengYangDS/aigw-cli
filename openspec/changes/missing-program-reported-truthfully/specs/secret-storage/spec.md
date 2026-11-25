## MODIFIED Requirements

### Requirement: Token availability is scoped to active demand

Readiness SHALL require a provider's Token only for models that clients use,
and client-native authentication SHALL remain independent. `aigw check` SHALL
pass a client's `local` depth before reading its Token or probing an endpoint,
and an unobserved Token SHALL NOT be reported as absent. `aigw model search`,
catalog checks and `aigw provider balance` SHALL read a selected provider's
Token when present.

#### Scenario: A client's configuration files are not ready

- **WHEN** `aigw check` finds a missing executable, missing target or invalid
  client configuration file for an enabled client
- **THEN** it reports that client's `local` failure and its fix
- **AND** the fix of a missing configured executable SHALL be
  `aigw client edit <client> --executable <path>`, never `aigw sync`
- **AND** it reads no Token value and makes no authenticated request for that
  client, even if its Token is missing or unreadable.

#### Scenario: A client's configuration files are ready

- **WHEN** the enabled client passes its `local` depth
- **THEN** a client that uses a provider Token reads it once before its
  `endpoint` depth, retaining credential failure details when the read fails
- **AND** client-owned authentication reads no AIGW credential and probes no
  endpoint.

#### Scenario: A team file contains unused providers

- **WHEN** a reviewed team file contains providers whose models no enabled
  client uses
- **THEN** their missing Tokens do not block setup, synchronization, status, or
  `aigw check` for the clients in use.

#### Scenario: A command reads a selected provider's Token

- **WHEN** `aigw model search`, `aigw provider balance` or a catalog check
  selects a provider, whether or not a client uses its models
- **THEN** it SHALL read that provider's Token when present
- **AND** a provider without a Token SHALL get its fix while the other selected
  providers still print, and the command SHALL exit 1, as for an unreadable
  catalog.
