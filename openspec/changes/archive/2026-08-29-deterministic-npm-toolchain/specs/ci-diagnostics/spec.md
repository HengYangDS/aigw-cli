## MODIFIED Requirements

### Requirement: Hosted Git initialization is explicit

Every hosted job that runs Git-aware tooling SHALL initialize and verify its
exact checkout, revision, platform, and repository-declared toolchain in the
provider's own environment. GitLab source verification SHALL obtain
standalone source tools from an authenticated project-local package bound to
the current `mise.lock` digest while retaining mise's checksum verification.
Both Forges SHALL materialize npm repository tools from the exact committed
`package-lock.json` before source verification. Release-only tools SHALL remain
outside the source closure.

#### Scenario: A hosted action initializes a repository

- **WHEN** checkout or a test fixture initializes Git state
- **THEN** Git resolves `main` as the default branch
- **AND** the repository-declared runtime and standalone tools are installed without peer-Forge API discovery
- **AND** npm repository tools are installed from the committed transitive lock with install scripts disabled
- **AND** GitLab Linux bootstrap is defined once and inherited by every consuming job
- **AND** GitLab source verification uses only its declared source-tool closure
- **AND** native acceptance can execute every tool in its declared command closure
- **AND** no verification or provenance gate is weakened

#### Scenario: A required runner is unavailable

- **WHEN** no admitted runner can execute a required platform gate
- **THEN** the pipeline fails or reports the unavailable gate within a bounded interval
- **AND** does not remain pending indefinitely
