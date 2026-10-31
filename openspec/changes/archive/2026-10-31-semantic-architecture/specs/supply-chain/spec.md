## Purpose

Define how the repository supplies its locked tools, dependencies, scanners and
official clients, and the evidence that binds each to its lock.

## ADDED Requirements

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

### Requirement: Native verification runs the declared Mise release

Local verification and every Forge job SHALL run the Mise release that the
repository declares. A Mise release whose diagnostics the repository's tests
assert SHALL become the CUE pin and the repository floor together, and a
runner-owned Mise SHALL be replaced before a job consumes that declaration.

#### Scenario: A Mise release changes an asserted diagnostic

- **WHEN** a repository test asserts a diagnostic that only a newer Mise release
  prints
- **THEN** the CUE pin, the generated Forge projections and the repository Mise
  floor SHALL name that release
- **AND** the hosted native jobs SHALL pass with it before integration.

#### Scenario: A runner owns its Mise

- **WHEN** the declared Mise advances while a runner owns its Mise executable,
  such as GitLab's Windows ARM64 runner
- **THEN** that runner's executable and shim SHALL be replaced with the verified
  release before any of its jobs consumes the declaration.

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

### Requirement: A vulnerable exact transitive pin is overridden

When a direct dependency pins a transitive release that a published advisory
places in its vulnerable range, the package manifest SHALL override that pin,
scoped to that dependency, with the highest fixed stable release. The override
SHALL leave when a release of the dependency requires a fixed release itself.

#### Scenario: A direct dependency pins a vulnerable release exactly

- **WHEN** an advisory places a release that a direct dependency pins exactly
  in its vulnerable range, such as re2js 2.8.6 in markdownlint 0.42.0
- **THEN** the manifest SHALL override that dependency's pin with the highest
  fixed stable release, without the ordinary release-age delay
- **AND** the lock SHALL change only for that override, and the repository's
  checks SHALL pass with it before integration.

#### Scenario: The dependency adopts a fixed release

- **WHEN** the refresh adopts a release of the dependency that requires a fixed
  release itself
- **THEN** the same refresh SHALL remove the override
- **AND** the lock SHALL resolve the dependency's own requirement.

### Requirement: Dependency evidence binds the selected lockfiles

The release scanner invocation and report admission SHALL share one exact
lockfile-path selection. Normalization SHALL admit every selected lockfile
exactly once with nonempty package observations and established license
identities before writing either report; an exact version-bound OSV license
override MAY correct a verified upstream metadata gap without suppressing
vulnerability observations. Scanning and admission SHALL precede artifact
construction.

#### Scenario: Dependency admission fails before construction

- **WHEN** the scanner fails, its output is malformed, or its scope is incomplete
- **THEN** GoReleaser, SBOM generation and signing SHALL NOT start
- **AND** accepted output and cleanup obligations SHALL remain unchanged.

#### Scenario: Scanner output does not establish the selected scope

- **WHEN** an OSV report is empty, partial, duplicated, attributed to another
  checkout or source kind, or lacks package observations
- **THEN** release construction SHALL fail before evidence signing
- **AND** neither normalized report SHALL be written
- **AND** accepted output SHALL remain unchanged and temporary construction
  state SHALL be reclaimed.

#### Scenario: Scanner reports an unestablished license

- **WHEN** an observed package has no license or reports a blank, `UNKNOWN`,
  `NOASSERTION`, or `NONE` license identity
- **THEN** release construction SHALL reject the dependency report before
  writing normalized evidence
- **AND** neither normalized license nor vulnerability evidence SHALL be signed.

#### Scenario: Locked package has incomplete upstream license metadata

- **WHEN** a maintainer verifies the license in an integrity-locked package but
  OSV reports an unestablished license
- **THEN** the selected OSV policy MAY override only that exact ecosystem,
  package name, and version
- **AND** the override SHALL leave vulnerability scanning active
- **AND** a later package version SHALL require a new license observation.

#### Scenario: The Go standard library is part of the scan

- **WHEN** release construction scans `go.mod`
- **THEN** the selected OSV policy SHALL keep the Go standard library at the
  module's Go version in the scanned inventory
- **AND** construction SHALL fail before signing unless each release program in
  the SBOM carries exactly one package of the main module that `go.mod`
  declares and exactly one standard library, at that version, as a dependency
  of that same program's main-module package.

### Requirement: Host-specific tooling builds only for its hosts

Repository tooling SHALL place behavior that only one family of hosts runs, such
as a POSIX process inventory, in files whose build constraints select those
hosts, with an explicit counterpart for the other hosts. Each native job's
statement coverage SHALL count only code that its host can run.

#### Scenario: Native client retirement checks for processes

- **WHEN** native client retirement checks whether a process still uses the
  supply
- **THEN** its `ps` inventory SHALL build only for Linux and macOS
- **AND** Windows SHALL build its own counterpart, which makes no inventory
- **AND** the Windows native job's coverage SHALL not count the POSIX
  inventory.

### Requirement: A dependency scan exit needs a normalized advisory

A dependency scan that exits with findings SHALL fail unless its retained
evidence holds at least one normalized advisory.

#### Scenario: The scanner reports findings that normalization cannot name

- **WHEN** OSV-Scanner exits 1 and normalization finds no advisory
- **THEN** the dependency gate SHALL fail
- **AND** its exit record SHALL mark the scan as failed.

### Requirement: Dependency scanning runs only the locked scanner image

Dependency scanning SHALL run only the scanner executable whose SHA-256 equals
the asset that the source's Mise lock selects for the host platform and which
reports the locked version, and the scan SHALL observe that same executable.

#### Scenario: The scanner on PATH is not the locked image

- **WHEN** the resolved scanner's SHA-256 differs from the lock's asset for the
  host platform, it reports another version, or its image changes between the
  version report and the scan
- **THEN** the scan SHALL fail before writing either normalized report
- **AND** release construction SHALL NOT start GoReleaser, SBOM generation or
  signing.

### Requirement: Dependency reports name their scanner

Both dependency reports SHALL name the scanner's tool, version, platform,
SHA-256 and host-independent arguments, and each scan SHALL retain a local
receipt of its exact invocation and its raw report's digest.

#### Scenario: A scan names its scanner

- **WHEN** a scan runs with the locked image
- **THEN** both reports SHALL name the same scanner, with the source root and
  the evidence directory written as `{source}` and `{evidence}`
- **AND** the receipt SHALL record the executable's path and SHA-256, its
  version report, the exact arguments and the raw report's SHA-256.

### Requirement: Release provenance binds the locked scanner

Release provenance SHALL list the locked scanner asset that schema 2 dependency
reports name, verification SHALL refuse reports that name another image, and
schema 1 reports SHALL name no scanner and contribute no scanner dependency.

#### Scenario: A release names its scanner

- **WHEN** release construction writes schema 2 reports
- **THEN** the provenance SHALL list the lock's asset URL and SHA-256 for the
  scanner they name.

#### Scenario: An earlier release is verified

- **WHEN** verification reads schema 1 reports that name no scanner
- **THEN** the provenance SHALL include no scanner dependency and SHALL verify
  as before
- **AND** a schema 2 report without a scanner, or reports that name different
  scanners, SHALL be refused.
