# ci-diagnostics Specification

## Purpose

Define quiet, deterministic hosted CI diagnostics that expose actionable
failures without relying on runner-global state.

## Requirements

### Requirement: Hosted Git initialization is explicit

Every hosted Git-aware job SHALL verify its checkout, revision, platform, and
repository-locked toolchain in the Forge environment. GitLab source
verification SHALL install standalone tools from an authenticated project
package bound to `mise.lock` while retaining checksum verification. Both Forges
SHALL install npm source tools from `package-lock.json`. Release-only tools
MUST remain outside the source closure.

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
