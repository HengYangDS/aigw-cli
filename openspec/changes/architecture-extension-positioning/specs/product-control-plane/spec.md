## ADDED Requirements

### Requirement: Composable extension boundary

AIGW SHALL keep local configuration and client projection independent from API
traffic processing. A compatible endpoint or model SHALL enter as Account data;
a distinct credential exchange SHALL extend Account authentication; a new local
client SHALL enter through one complete Client Adapter; and incompatible wire
behavior SHALL remain in an independently operated data-plane product. An
external gateway or compatibility service MUST remain an optional Account
endpoint and MUST NOT become an AIGW runtime dependency.

A mature dependency SHALL be admitted only when it preserves these authority
boundaries and removes more owned implementation, tests, dependencies, and
operational states than it introduces.

#### Scenario: Add a compatible Provider endpoint

- **WHEN** an endpoint satisfies an admitted client protocol and existing
  Account authentication contract
- **THEN** an operator SHALL add it through configuration data
- **AND** AIGW SHALL NOT add provider-name branching or a provider-specific
  runtime package.

#### Scenario: Add a client integration

- **WHEN** a new local client requires AIGW-managed configuration
- **THEN** it SHALL be admitted through its own discovery, planning, guarded
  projection, verification, rollback, and uninstall boundary
- **AND** it SHALL NOT reuse another client's conditional path or private state.

#### Scenario: Compose with a traffic gateway

- **WHEN** an operator selects a general gateway or narrow compatibility
  service
- **THEN** AIGW SHALL model its URL as an ordinary Account endpoint
- **AND** SHALL NOT install, supervise, configure, embed, or copy the service's
  traffic policy.

#### Scenario: Evaluate a mature dependency

- **WHEN** a library or framework is proposed for an AIGW-owned boundary
- **THEN** admission SHALL demonstrate a net reduction in owned complexity
- **AND** popularity or feature count alone SHALL NOT justify adoption.
