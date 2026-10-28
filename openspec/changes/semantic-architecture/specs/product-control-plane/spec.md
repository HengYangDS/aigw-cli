## MODIFIED Requirements

### Requirement: Enforced semantic ownership and quality

Each behavior and policy SHALL have one semantic owner. Composition roots SHALL
assemble declared owners; source gates SHALL enforce positive package topology,
dependency direction, public surfaces, portability, and the canonical coverage
policy. Compatibility facades and duplicate policy owners SHALL not be retained.

#### Scenario: semantic ownership regresses

- **WHEN** a change violates declared topology or dependency direction, or misses the canonical coverage policy
- **THEN** verification SHALL fail with the exact semantic owner and evidence gap.

#### Scenario: Architecture or coverage regresses

- **WHEN** a change violates declared semantic ownership or dependency direction, or misses the canonical package or aggregate coverage policy
- **THEN** local and hosted verification SHALL fail before publication.

#### Scenario: Foreign-host absolute path enters policy

- **WHEN** policy contains an absolute or parent-traversing path in another host's syntax
- **THEN** validation SHALL reject it identically on macOS, Linux, and Windows.

#### Scenario: No admitted branch authority exists

- **WHEN** no maintained admitted analyzer can measure a proposed branch metric
- **THEN** the quantitative policy SHALL omit that unsupported claim
- **AND** native statement evidence and complete package observation SHALL remain
  enforced without being relabeled as branch coverage.

#### Scenario: A tool needs shared release policy

- **WHEN** repository release tooling and product update read the same release artifact format
- **THEN** both import that format from its one product owner
- **AND** each still validates its own authority-bound inputs: the release tool its source and lock, product update its trust anchors and downloaded bytes.

#### Scenario: A legacy concatenated name remains

- **WHEN** a package appears outside the declared direct-owner topology or a repository tool imports a product package outside its admitted layers
- **THEN** the architecture gate fails with the exact path and dependency.

#### Scenario: an ordinary provider is added

- **WHEN** a provider is added below the existing provider owner without changing topology
- **THEN** the existing positive topology admits it without changing repository-shape policy.

### Requirement: Composable extension boundary

AIGW SHALL keep configuration and client projection separate from API traffic.
Endpoints and models SHALL enter as Account data; existing
client-native authentication SHALL be reused through explicit Routes. New
clients SHALL require complete Adapters, unsupported credential contracts SHALL
require admission, and incompatible wire behavior SHALL remain in a separate
data plane. Gateways SHALL remain optional endpoints. Dependencies MAY be
admitted only when total owned complexity falls.

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

#### Scenario: A client adapter is registered

- **WHEN** an adapter is registered, including a test-only rehearsal adapter
- **THEN** it SHALL pass one shared adapter contract: its plan lists every
  file that applying it may change and states exactly whether applying it
  changes one, rollback restores the prior bytes and modes, withdrawal leaves
  no owned file, and inspection, verification and planning change no file and
  leave the caller's configuration unchanged
- **AND** an adapter that fails any part of that contract SHALL NOT be admitted.

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
