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

- **WHEN** no stable tool can measure the complete module once on every supported platform
- **THEN** the branch-coverage gate SHALL remain blocked rather than substituting statement coverage.

#### Scenario: A tool needs shared release policy

- **WHEN** repository release tooling and product upgrade behavior require the same source-validation rule
- **THEN** each validates its own authority-bound inputs without importing another runtime owner.

#### Scenario: A legacy concatenated name remains

- **WHEN** a package appears outside the declared direct-owner topology or a repository tool imports an undeclared product owner
- **THEN** the architecture gate fails with the exact path and dependency.

#### Scenario: an ordinary provider is added

- **WHEN** a provider is added below the existing provider owner without changing topology
- **THEN** the existing positive topology admits it without changing repository-shape policy.
