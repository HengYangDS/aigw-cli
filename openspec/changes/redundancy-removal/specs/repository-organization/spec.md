## MODIFIED Requirements

### Requirement: Portable repository quality surface

Repository configuration, tools, source, tests, documentation, and release assets SHALL have one semantic owner. Native tool settings SHALL have one consuming carrier; repository parameters SHALL have one owning policy. Every quality and release reader SHALL use its declared carrier and refuse absent or malformed required settings. Unrelated consumers SHALL own adaptation and qualification outside this product's implementation and quality graph.

#### Scenario: A contributor follows one behavior

- **WHEN** a contributor traces a public command or projection
- **THEN** implementation, tests, specification, and documentation identify one owner
- **AND** no compatibility facade or duplicate configuration must be consulted.

#### Scenario: Physical layout introduces competing authority

- **WHEN** root-level sprawl, forwarding scripts, concatenated package names, cross-package private calls, or duplicated policy create a parallel authority
- **THEN** the content is organized under its semantic owner
- **AND** those structures do not remain as a competing implementation or policy.

#### Scenario: Contributor uses another host

- **WHEN** the repository is verified on macOS, Linux, or Windows
- **THEN** repository-owned Go commands and declarative configuration provide the same contract
- **AND** shell-specific behavior is not required for product or CI correctness.

#### Scenario: An unrelated consumer reads product material

- **WHEN** an external product consumes selected repository material
- **THEN** its adaptation and output remain outside this repository
- **AND** AIGW qualification does not require that external product or its tests.

#### Scenario: A native configuration carrier moves

- **WHEN** an admitted migration moves tool settings to their consuming owner
- **THEN** every current native reader, fixture and documented entry uses that owner
- **AND** parsed settings, quality thresholds and generated outputs retain their accepted meaning
- **AND** immutable historical originals remain with their evidence consumers.

#### Scenario: A required formatter configuration is unavailable

- **WHEN** the native formatter fix or check command cannot read its declared settings
- **THEN** the command fails before changing any authored input
- **AND** it does not substitute ambient configuration or defaults.

#### Scenario: A peer receives a branch or tag

- **WHEN** a branch or release tag is published to a Forge peer
- **THEN** ETHOS publishes it
- **AND** repository tools only verify the published Forge objects.

#### Scenario: A repository command has no consumer

- **WHEN** a repository tool subcommand, option, field or reader has no
  production, CI, documentation or operator consumer
- **THEN** it is removed rather than retained beside its owner.
