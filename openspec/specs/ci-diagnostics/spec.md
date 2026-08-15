# ci-diagnostics Specification

## Purpose

Define quiet, deterministic hosted CI diagnostics that expose actionable
failures without relying on runner-global state.
## Requirements
### Requirement: Hosted Git initialization is explicit

Every hosted job that runs Git-aware tooling SHALL initialize and verify its
exact checkout, revision, platform, and repository-declared toolchain in the
provider's own environment. GitLab Linux jobs SHALL inherit one toolchain
template that obtains the exact repository-declared mise version from the
current project's authenticated Generic Package Registry, verifies the
architecture-specific immutable digest, and completes before any repository
command. The GitLab source job SHALL additionally declare one exact tool
closure and obtain peer-Forge-hosted source tools from an authenticated
project-local package bound to the current `mise.lock` digest; its manifest and
every asset SHALL retain mise's locked checksum verification. Release-only tools SHALL not
enter that closure. Native jobs SHALL enable the complete, minimal runtime tool
closure of their acceptance command. GitHub verification and release workflows SHALL
declare `main` as Git's process-scoped default branch without changing
runner-global configuration. Runner names, labels, and workspace paths SHALL
follow project and platform semantics rather than a personal host layout or the
peer Forge's conventions.

#### Scenario: A hosted action initializes a repository

- **WHEN** checkout or a test fixture initializes Git state
- **THEN** Git resolves `main` as the default branch
- **AND** the repository-declared toolchain is installed without peer-Forge API discovery
- **AND** GitLab Linux bootstrap is defined once and inherited by every consuming job
- **AND** GitLab source verification uses only its declared, lock-bound source tool closure
- **AND** native acceptance can execute every tool in its declared command closure
- **AND** no verification or provenance gate is weakened

#### Scenario: A required runner is unavailable

- **WHEN** no admitted runner can execute a required platform gate
- **THEN** the pipeline fails or reports the unavailable gate within a bounded interval
- **AND** does not remain pending indefinitely
