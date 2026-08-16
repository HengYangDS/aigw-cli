## MODIFIED Requirements

### Requirement: semantic structure

Production, test, and repository-tool code SHALL follow the declared semantic
topology and dependency direction. Naming, package documentation, import
ownership, and composition roots SHALL express cohesive semantic owners. Shared
behavior SHALL live at the smallest stable owner rather than in a forwarding
wrapper, alias-only package, or copied helper. Size, complexity, nesting, and
other presentation heuristics MAY inform review, but SHALL NOT reject a change
without an independently justified risk model, defined measurement semantics,
false-positive cost, remediation path, and review trigger.

#### Scenario: structure violates semantic ownership

- **WHEN** production, test, or tool code violates the declared topology,
  dependency direction, composition root, naming, or ownership contract
- **THEN** the architecture gate SHALL fail with the exact semantic violation.

#### Scenario: a heuristic changes

- **WHEN** a size, complexity, nesting, or presentation heuristic is proposed
  as a merge condition
- **THEN** it SHALL remain review evidence unless its risk model, measurement,
  false-positive cost, remediation path, and review trigger are admitted.

#### Scenario: an ordinary provider is added

- **WHEN** an ordinary provider implementation is added below the declared
  provider owner without changing package topology or dependency direction
- **THEN** no repository-shape allowance or threshold change SHALL be required.
