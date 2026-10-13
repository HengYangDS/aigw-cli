## MODIFIED Requirements

### Requirement: Portable repository quality surface

Repository configuration, tools, source, tests, documentation, and release
assets SHALL be organized by semantic owner. Root-level sprawl, forwarding
scripts, concatenated package names, cross-package private calls, and duplicated
policy SHALL not create parallel authority. Unrelated external consumers SHALL
own their adaptation and qualification outside this product's implementation
and quality dependency graph.

#### Scenario: A contributor follows one behavior

- **WHEN** a contributor traces a public command or projection
- **THEN** implementation, tests, specification, and documentation identify one owner
- **AND** no compatibility facade or duplicate configuration must be consulted.

#### Scenario: Contributor uses another host

- **WHEN** the repository is verified on macOS, Linux, or Windows
- **THEN** repository-owned Go commands and declarative configuration provide the same contract
- **AND** shell-specific behavior is not required for product or CI correctness.

#### Scenario: An unrelated consumer reads product material

- **WHEN** an external product consumes selected repository material
- **THEN** its adaptation and output remain outside this repository
- **AND** AIGW qualification does not require that external product or its tests.
