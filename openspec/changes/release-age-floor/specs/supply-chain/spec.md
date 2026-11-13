## MODIFIED Requirements

### Requirement: Latest stable repository-owned supply chain

AIGW SHALL lock current stable Go, tool, Action, and release dependencies
through one repository-owned authority for each ecosystem. The dependency
policy SHALL declare one minimum release age, in hours, for every source; only a
security fix skips it. The declared Go toolchain and resolver SHALL own
transitive closure; local verification and both Forge projections SHALL consume
those declarations rather than duplicate version literals or fallbacks.

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

#### Scenario: A new release is still inside the minimum release age

- **WHEN** the newest stable release of a declared dependency was published
  less than the repository's minimum release age ago, and no advisory names the
  locked release
- **THEN** the refresh SHALL keep the locked release and name the held release
  and the time when its age ends
- **AND** a later refresh SHALL take it once that age has passed.

#### Scenario: A release publishes its packages at different times

- **WHEN** a release ships as several packages that the registry records with
  different publish times, such as a native client's platform packages
- **THEN** its minimum release age SHALL run from the latest publish time among
  the packages that the supplying hosts install
- **AND** a refresh SHALL NOT pin the release before that age has passed for
  each of those packages.

#### Scenario: A release comes from a source that cannot replace it

- **WHEN** the newest stable release of a dependency comes from a source that
  cannot replace a published artifact under the same identity, such as a Go
  module in the checksum database, a Mise tool locked by checksum, a container
  image pinned by digest or an Action pinned by commit
- **THEN** the refresh SHALL hold it for the declared minimum release age, as
  it holds a release from a registry that can still unpublish one, such as npm
- **AND** the lock SHALL bind the checksum, digest or commit that its publisher
  serves.

#### Scenario: The minimum release age changes

- **WHEN** the minimum release age is raised or lowered
- **THEN** one value of the dependency policy SHALL change, written as a whole
  number of hours, and every source SHALL follow it
- **AND** the repository's dependency checks SHALL fail while another rule
  declares an age of its own, or while a security fix does not skip the age.

#### Scenario: A registry publishes no release time

- **WHEN** the minimum release age is above zero and the registry of a pinned
  image publishes no time for a newer release, such as a container registry
  outside Docker Hub
- **THEN** the candidate calculation SHALL hold that release rather than report
  it as an update
- **AND** the refresh SHALL take it only after its publisher's own release
  record shows that it is past the age.

#### Scenario: A stable release breaks a declared resolution mode

- **WHEN** the newest stable release of a tool makes a resolution mode that
  the repository's verification declares fail, such as Mise 2026.11.0 refusing
  a locked npm tool in safe mode
- **THEN** the refresh SHALL keep the locked release and name the failing mode,
  the owner of the hold and its exit condition
- **AND** it SHALL NOT disable that mode or patch the tool to make the release
  pass.

#### Scenario: An advisory names a dependency that a locked tool bundles

- **WHEN** a published advisory names a dependency that the locked release of
  a tool bundles, such as npm bundling undici, and a newer release of that tool
  bundles a fixed version
- **THEN** the declaration and its lock SHALL advance to that tool release as for
  an advisory that names the tool itself.
