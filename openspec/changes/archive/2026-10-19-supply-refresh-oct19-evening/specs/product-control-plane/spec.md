## MODIFIED Requirements

### Requirement: Latest stable repository-owned supply chain

AIGW SHALL lock current stable Go, tool, Action, and release dependencies
through one repository-owned authority for each ecosystem. The declared Go
toolchain and resolver SHALL own transitive closure; local verification and both
Forge projections SHALL consume those declarations rather than duplicate version
literals or compatibility fallbacks.

#### Scenario: A stable transitive update is available

- **WHEN** the Go resolver reports a newer stable transitive dependency
- **THEN** the owning direct dependency and native resolver SHALL determine its
  admissible closure; an unused transitive update SHALL NOT create a new pin
- **AND** an admitted graph change SHALL refresh declarations and locks together
  and pass complete native verification before integration

#### Scenario: A preceding archive projection changes text layout

- **WHEN** an OpenSpec archive projection leaves a surplus terminal blank line
- **THEN** the same native gate SHALL reject it
- **AND** the active closeout SHALL restore canonical text without weakening policy

#### Scenario: A declared stable dependency advances

- **WHEN** the locked supply chain is refreshed
- **THEN** local development, GitLab, and GitHub resolve the same declared versions
- **AND** obsolete pins and compatibility fallbacks are removed.

#### Scenario: A runner-owned tool advances with its declaration

- **WHEN** a refreshed declaration moves a tool whose executable a CI runner
  owns, such as the Windows ARM64 Mise
- **THEN** the CUE authority SHALL pin that release's version and executable
  digests from the verified upstream release
- **AND** the runner's executable SHALL be replaced in a window without running
  jobs before any job consumes the new declaration.

#### Scenario: An admitted tool release fails its own consistency check

- **WHEN** a native job reports that the admitted release of a tool is internally
  inconsistent, such as a Mise executable finding its own release's shim stale
- **THEN** the declaration SHALL advance to the stable release that fixes it, or
  return to the previous release, on every Forge projection and runner
- **AND** the tool's warning SHALL be absent from the next native jobs.
