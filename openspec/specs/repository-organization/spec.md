# repository-organization Specification

## Purpose
TBD - created by archiving change aigw-repository-organization-convergence. Update Purpose after archive.
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
through the repository's public governance lifecycle. GitLab and GitHub SHALL
publish the same accepted source independently; remote availability SHALL not be
a prerequisite for local proof or release assembly.

#### Scenario: Accepted content is ready for release

- **WHEN** exact-head proof has admitted the candidate and accepted `dev` is current
- **THEN** the governed release transition advances `main` from that accepted content
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
