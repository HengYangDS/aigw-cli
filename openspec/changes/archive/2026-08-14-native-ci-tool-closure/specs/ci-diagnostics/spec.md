## MODIFIED Requirements

### Requirement: Hosted Git initialization is explicit

Every hosted job that runs Git-aware tooling SHALL initialize and verify its
exact checkout, revision, platform, and repository-declared toolchain in the
provider's own environment. Container bootstrap SHALL install the exact
repository-declared mise version without release discovery. Native jobs SHALL
enable the complete, minimal runtime tool closure of their acceptance command.
GitHub verification and release workflows SHALL declare `main` as Git's
process-scoped default branch without changing runner-global configuration.
Runner names, labels, and workspace paths SHALL follow project and platform
semantics rather than a personal host layout or the peer Forge's conventions.

#### Scenario: A hosted action initializes a repository

- **WHEN** checkout or a test fixture initializes Git state
- **THEN** Git resolves `main` as the default branch
- **AND** the repository-declared toolchain is installed without peer-Forge API discovery
- **AND** native acceptance can execute every tool in its declared command closure
- **AND** no verification or provenance gate is weakened

#### Scenario: A required runner is unavailable

- **WHEN** no admitted runner can execute a required platform gate
- **THEN** the pipeline fails or reports the unavailable gate within a bounded interval
- **AND** does not remain pending indefinitely
