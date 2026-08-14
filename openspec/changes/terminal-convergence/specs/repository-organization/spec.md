## MODIFIED Requirements

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
