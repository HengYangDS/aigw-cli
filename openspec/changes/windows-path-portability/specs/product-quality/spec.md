## MODIFIED Requirements

### Requirement: portable repository text

The repository SHALL define editor intent, checkout normalization, quality
execution, build, installation, and release behavior using tracked,
platform-neutral configuration. The contract SHALL run through the same
repository-native command on macOS, Linux, and Windows without requiring a
particular interactive shell, personal path, workstation product, IDE, or
foreign repository.

Repository-owned paths SHALL use slash-separated repository semantics until the
filesystem boundary. A host adapter SHALL convert them exactly once after
rejecting traversal, absolute, backslash, and volume-qualified input. Tool
processes SHALL use repository-relative inputs from an explicit repository
working directory when absolute paths can cross host volumes.

#### Scenario: a contributor uses another supported host

- **WHEN** the repository is cloned under another operating system, user, directory, or Forge
- **THEN** checkout, verification, build, installation, and release contracts SHALL remain discoverable and executable from repository-owned inputs

#### Scenario: a contributor uses another operating system

- **WHEN** Git checks out tracked text
- **THEN** line endings and executable semantics SHALL remain deterministic

#### Scenario: Native Windows renders the CI projection

- **WHEN** the repository and a test fixture reside on different Windows volumes
- **THEN** CUE evaluates `.config/ci/pipeline.cue` from the selected repository root
- **AND** generated `.github` and GitLab paths resolve within that root
- **AND** the same focused contracts pass on macOS, Linux, and Windows
