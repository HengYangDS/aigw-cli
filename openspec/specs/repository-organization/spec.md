# repository-organization Specification

## Purpose

Define the repository's product-version, lifecycle, semantic ownership,
documentation, and portable quality boundaries so every governed surface has
one discoverable authority.

## Requirements

### Requirement: One repository version source

AIGW SHALL expose one tracked, machine-readable product version source used by CLI version output, changelog validation, artifact naming, and release checks.

#### Scenario: Local build reads the product version

- **WHEN** a contributor runs the repository-native build or version check
- **THEN** the command reads the same tracked version source without requiring a Forge tag or personal environment variable

#### Scenario: Version metadata disagrees

- **WHEN** a tag, changelog heading, generated artifact, or build input disagrees with the tracked version source
- **THEN** the repository gate fails before packaging or publication

### Requirement: Governed release-branch convergence

Local `candidate/dev`, protected `dev`, and protected `main` SHALL advance
through the repository's public governance lifecycle. The tracked branch-role
policy SHALL declare one `accepted-to-release` transition from `accepted_root`
to `release_root`, require `proof:execution`, and grant only the
`repository.release` capability. GitLab and GitHub SHALL publish the same
accepted source independently; remote availability SHALL not be a prerequisite
for local proof or release assembly.

#### Scenario: Accepted content is ready for release

- **WHEN** exact-head proof has admitted the candidate and accepted `dev` is current
- **THEN** the declared governed release transition advances `main` from that accepted content
- **AND** local readiness remains distinct from remote publication.

#### Scenario: Direct branch mutation is attempted

- **WHEN** an actor attempts to bypass the governed lifecycle for `candidate/dev`, `dev`, or `main`
- **THEN** repository admission blocks the mutation
- **AND** reports the required public governance operation.

#### Scenario: One Forge is unavailable

- **WHEN** one publication plane cannot be reached
- **THEN** the other may publish and verify the same signed revision independently
- **AND** local accepted state remains valid without either remote.

### Requirement: Portable repository quality surface

Repository configuration, tools, source, tests, documentation, and release
assets SHALL be organized by semantic owner. Root-level sprawl, forwarding
scripts, concatenated package names, cross-package private calls, and duplicated
policy SHALL not create parallel authority.

#### Scenario: A contributor follows one behavior

- **WHEN** a contributor traces a public command or projection
- **THEN** implementation, tests, specification, and documentation identify one owner
- **AND** no compatibility facade or duplicate configuration must be consulted.

#### Scenario: Contributor uses another host

- **WHEN** the repository is verified on macOS, Linux, or Windows
- **THEN** repository-owned Go commands and declarative configuration provide the same contract
- **AND** shell-specific behavior is not required for product or CI correctness.

### Requirement: Semantic documentation architecture

Documentation SHALL have one global entry point and semantic organization.
Official OpenSpec artifacts are the sole tracked change-intent authority; ETHOS
may derive only a transient Commitment containing `schema_version`, `id`, and
`acceptance`. Filenames MUST name their subjects. Local indexes or extra
carriers MAY exist only for semantics not representable by OpenSpec, the global
entry point, or existing authorities, and MUST declare owner, consumer,
replaced authority, and retirement.

#### Scenario: Reader enters the documentation

- **WHEN** a reader starts at `docs/README.md`
- **THEN** the entry point SHALL expose task-oriented paths and the complete
  information-domain map
- **AND** every canonical document SHALL be reachable from that map or a named
  semantic register.

#### Scenario: A document has a single semantic owner

- **WHEN** a document describes architecture, concepts, decisions, evidence,
  experience, governance, guidance, or operations
- **THEN** its directory and filename SHALL identify that owner
- **AND** no compatibility copy or redirect-only document SHALL remain.

#### Scenario: A directory contains multiple documents

- **WHEN** a semantic directory gains another document
- **THEN** file count alone SHALL NOT require a local `README.md`
- **AND** navigation SHALL remain with the smallest content-bearing owner.

#### Scenario: A repository gate consumes a semantic register

- **WHEN** a quality gate validates a documentation register
- **THEN** it SHALL consume the register's semantic filename
- **AND** it SHALL NOT require a container-named compatibility carrier.

#### Scenario: Governance evaluates change intent

- **WHEN** ETHOS evaluates the selected OpenSpec change
- **THEN** it SHALL compile the Commitment transiently from official OpenSpec
  artifacts
- **AND** the repository SHALL persist no parallel Commitment carrier.

#### Scenario: Historical change evidence is inspected

- **WHEN** a maintainer inspects an archived change
- **THEN** official OpenSpec archives and Git history SHALL describe the tracked
  change
- **AND** ETHOS Attestations SHALL remain the effect-evidence surface.
