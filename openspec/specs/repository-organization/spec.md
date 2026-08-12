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

The repository SHALL distinguish local `dev` acceptance from release `main` and advance `main` only through ETHOS accepted-root closeout.

#### Scenario: Accepted content is ready for release

- **WHEN** `candidate/dev` is clean, proved, and equal to accepted `dev`
- **THEN** the public closeout command may fast-forward `main` with exact-head checks and emits a machine-readable receipt

#### Scenario: Direct branch mutation is attempted

- **WHEN** a tool attempts to advance `main` outside the governed closeout contract
- **THEN** admission fails closed and no ref changes

### Requirement: Portable repository quality surface

The repository SHALL provide one documented local quality graph shared by CI and ETHOS proof, with editor and text normalization declared in tracked files.

#### Scenario: Contributor uses another host

- **WHEN** the repository is cloned under another supported OS or directory
- **THEN** quality commands and tracked text semantics remain discoverable and do not depend on a personal path, shell, identity, or Forge
