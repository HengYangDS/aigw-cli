## ADDED Requirements

### Requirement: Client executable fixtures are test-owned

Native tests SHALL construct client executables from test-owned
fixtures rather than borrowing unrelated host toolchain executables.

#### Scenario: A test exercises an unreadable client executable

- **WHEN** native verification exercises client-executable read failures
- **THEN** the fixture SHALL be an isolated executable controlled by the test
- **AND** the result SHALL not depend on the installed Go toolchain path or contents.

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

#### Scenario: Several client adapters repeat one step

- **WHEN** two or more client adapters implement the same step, such as a
  Converge preamble or a document encoder
- **THEN** that step SHALL have one owner that the adapters call
- **AND** an adapter SHALL keep its own sequence only where its order or
  error text differs.

#### Scenario: Product packages and repository tools repeat one step

- **WHEN** product packages or repository tools implement the same step, such
  as a file digest, a process runner or an authorized download
- **THEN** that step SHALL have one owner in a package that every consumer may
  import
- **AND** no consumer SHALL keep a private copy, a forwarding wrapper or a hook
  that only tests set.

#### Scenario: Contributor uses another host

- **WHEN** the repository is verified on macOS or Linux
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

### Requirement: Command packages mirror the command tree

Each top-level command SHALL be implemented by one package named after it.
Acceptance scripts SHALL run the program in its own process for every
command's help, a representative success, a representative refusal and its
JSON output on macOS and Linux.

#### Scenario: A command has no package or script

- **WHEN** a top-level command is registered without a package of its name or
  without acceptance scripts
- **THEN** the repository gate fails and names the command.

#### Scenario: Command output changes

- **WHEN** a change alters a command's help, exit status or JSON output
- **THEN** that command's acceptance script fails until the same change
  updates the script.

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
- **THEN** validation SHALL reject it identically on macOS and Linux.

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

## REMOVED Requirements

### Requirement: Native client fixtures are repository-controlled

**Reason**: Its only scenario named Windows verification, and OpenSpec keeps every scenario of a modified requirement.

**Migration**: "Client executable fixtures are test-owned" keeps the rule for macOS and Linux.
