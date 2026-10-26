## ADDED Requirements

### Requirement: Real-client acceptance supplies each host's official clients

Real-client acceptance on a native host SHALL first supply exactly the clients
that the release owner declares for that platform, from their official
distributions at the pinned releases, and export each client's explicit
executable. Supply state SHALL stay in one job-owned directory that every
outcome removes. A missing or unverifiable distribution SHALL fail the supply
rather than narrow the journey.

#### Scenario: A Linux or macOS job requests real clients

- **WHEN** a GitHub native Linux or macOS job runs the retained-predecessor
  journey with real clients
- **THEN** it SHALL supply Codex and Claude Code from their official npm
  platform packages, and Hermes from its official release installer after that
  installer's pinned checksum matches
- **AND** the installed Hermes checkout SHALL be the unmodified pinned commit
  with its published launcher
- **AND** a Linux job SHALL first install the sandbox prerequisites that Codex
  documents for its host, the distribution's bubblewrap and its AppArmor
  profile, without lifting the host's user-namespace restriction.

#### Scenario: A Windows job requests real clients

- **WHEN** a GitHub native Windows job runs the same journey with real clients
- **THEN** it SHALL supply Codex and Claude Code from the same pinned releases
  with Git Bash for Claude
- **AND** it SHALL refuse Hermes inputs, because that host's journeys omit
  Hermes.

#### Scenario: A supply fails or is interrupted

- **WHEN** an installation step fails, a pinned identity does not match, or the
  job is interrupted during supply
- **THEN** the supply SHALL stop the running installer and every process it
  started, without their cooperation
- **AND** it SHALL export no client and SHALL remove its directory.

#### Scenario: The job environment names other client state

- **WHEN** the job environment sets a client's home or tool directory, such as
  `HERMES_RUNTIME_DIR`
- **THEN** the official installer SHALL still keep its homes, tools, caches and
  launcher inside the supply.

#### Scenario: The job retires its supply

- **WHEN** the native job ends after acceptance, whatever its outcome
- **THEN** retirement SHALL remove only the recorded supply directory and prove
  it absent
- **AND** on Linux and macOS a process that still names the supply SHALL keep
  the directory and fail the job.

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

#### Scenario: A security advisory names a locked tool

- **WHEN** a published advisory places the locked release of a tool in its
  vulnerable range, such as uv before 0.13.4 trusting a compromised CA
- **THEN** the declaration and its lock SHALL advance to the first fixed stable
  release without the ordinary release-age delay
- **AND** the advance SHALL pass complete native verification before
  integration.

#### Scenario: A security advisory names the toolchain in released programs

- **WHEN** an advisory places the locked Go standard library in its vulnerable
  range, such as Go before 1.27.3 trusting compromised intermediate certificates
- **THEN** the Mise declaration, the `go.mod` directive and the
  standard-library scan override SHALL advance together to the first fixed
  stable toolchain
- **AND** the next AIGW release SHALL be built with it, because released
  programs embed the standard library.

#### Scenario: A qualified native client release advances

- **WHEN** the admitted native client releases advance past their pinned
  versions and the repository's minimum release age
- **THEN** the CUE authority SHALL pin the exact new versions once, and both
  Forge projections, every native host's client supply and the Windows client
  supplier name SHALL follow from it
- **AND** the native client journey of each supplying host SHALL pass with the
  new versions before the forges integrate them.

#### Scenario: A candidate lookup cannot reach its datasource

- **WHEN** the candidate calculation cannot reach the datasource of a declared
  dependency, such as the Go module proxy or a container registry
- **THEN** that dependency SHALL remain unaudited rather than current, and the
  failed lookups SHALL be retained as the calculation's evidence
- **AND** a later calculation that reaches the datasource SHALL audit it before
  the supply chain is described as current.
