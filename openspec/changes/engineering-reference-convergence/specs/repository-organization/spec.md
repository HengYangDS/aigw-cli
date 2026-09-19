# Spec Delta

## ADDED Requirements

### Requirement: Repository meaning is traceable through one semantic path

Every public behavior, invariant, configuration rule, and extension point SHALL have one discoverable semantic owner linking its specification, implementation, tests, operational guidance, and quality enforcement. A contributor SHALL NOT need a compatibility alias, filename convention, private local artifact, or duplicate policy carrier to reconstruct that meaning.

#### Scenario: A contributor traces a product behavior

- **WHEN** a contributor starts from a public command, user journey, or documented invariant
- **THEN** repository navigation SHALL lead to one owning package or tool, its focused tests, its canonical specification, and its operational documentation
- **AND** sibling files SHALL not implement an undisclosed parallel path.

#### Scenario: Physical organization contradicts semantic ownership

- **WHEN** flat suffix families, ambiguous directories, mixed responsibilities, or misplaced documents conceal one reason to change
- **THEN** the repository SHALL move the content to the smallest cohesive semantic owner
- **AND** delete superseded paths rather than retain forwarding shells or compatibility copies.

### Requirement: A clean checkout reproduces the contribution path

A supported contributor SHALL reconstruct each Work Lane's mutable development state from committed locks through one documented repository entrypoint. The resulting environment SHALL expose the same format, lint, type, test, security, architecture, documentation, and generation contracts used by CI without depending on a sibling checkout or ambient system package version.

#### Scenario: A new contributor changes one invariant

- **WHEN** a contributor starts from a clean checkout and follows the documented bootstrap and verification path
- **THEN** the declared environment SHALL be reconstructed without manual dependency discovery
- **AND** the contributor SHALL be able to locate, change, test, and review one semantic owner using repository commands.
