## MODIFIED Requirements

### Requirement: Latest stable repository-owned supply chain

AIGW SHALL lock current stable Go, tool, Action, and release dependencies
through one repository-owned authority for each ecosystem. Mise's `minimum_release_age` SHALL declare the one release age, in whole hours;
every other source SHALL hold it; Renovate skips it for a security fix. The declared Go toolchain and resolver SHALL own
transitive closure; local verification and both Forge projections SHALL consume
those declarations rather than duplicate version literals or compatibility
fallbacks.

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
  owns, such as GitLab's macOS ARM64 Mise
- **THEN** the CUE authority SHALL pin that release's version and executable
  digests from the verified upstream release
- **AND** the runner SHALL hold the verified executable before any job consumes
  the new declaration, without changing the executable that a running job uses.

#### Scenario: A refresh has no window for a runner-owned tool

- **WHEN** the candidate calculation reports a newer release of a tool whose
  executable a CI runner owns, and the runner cannot hold that release before
  the refresh's proposal is published
- **THEN** the refresh SHALL keep the pinned release
- **AND** it SHALL name the Change that moves the declaration together with
  the runner's installation, with an owner and a date.

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
  Forge projections and every native host's client supply SHALL follow from it
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
- **THEN** one value, Mise's `minimum_release_age`, SHALL change, written as a
  whole number of hours, and `mise run fix` SHALL write it into every other
  source
- **AND** the repository's dependency checks SHALL fail while another rule
  declares an age of its own, or while a security fix does not skip the age.

#### Scenario: A source holds another age than the declared one

- **WHEN** Renovate's root age or npm's `min-release-age` differs from the
  declared age, which npm holds in whole days rounded up and, at zero, leaves
  out
- **THEN** the repository's dependency checks SHALL fail and name that file
- **AND** `mise run fix` SHALL write the declared age into it.

#### Scenario: A security fix is younger than the declared age

- **WHEN** a security fix is published inside the declared age
- **THEN** Renovate's security rule SHALL propose it without waiting
- **AND** npm SHALL hold it for the age, because the repository admits no npm
  exemption, and a change that takes it sooner SHALL say so.

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

### Requirement: Every native job runs the declared Mise release

Local verification and every Forge job SHALL run the Mise release that the
repository declares. A Mise release whose diagnostics the repository's tests
assert SHALL become the CUE pin and the repository floor together, and a
runner-owned Mise SHALL be installed and verified before a job consumes that
declaration.

#### Scenario: A Mise release changes an asserted diagnostic

- **WHEN** a repository test asserts a diagnostic that only a newer Mise release
  prints
- **THEN** the CUE pin, the generated Forge projections and the repository Mise
  floor SHALL name that release
- **AND** the hosted native jobs SHALL pass with it before integration.

#### Scenario: A runner owns its Mise

- **WHEN** the declared Mise advances while a runner owns its Mise executable,
  such as GitLab's macOS ARM64 runner
- **THEN** the runner SHALL hold the verified release in a directory that names
  that release, beside the release of the earlier declaration, before any of
  its jobs consumes the new declaration
- **AND** the runner SHALL remove the earlier release once no job consumes the
  earlier declaration, and not before.

#### Scenario: Projects share a macOS runner host

- **WHEN** the runners of several projects share a macOS ARM64 host whose shared
  Mise no single project's declaration governs
- **THEN** each of AIGW's GitLab jobs there SHALL verify the digest and version
  of a Mise executable in a directory that only AIGW's declaration updates,
  before any locked tool runs, and SHALL print the release it verified
- **AND** the word `mise` SHALL run that executable for the rest of the job,
  whatever the runner account's shell profile defines
- **AND** a missing executable, another digest or another version SHALL fail the
  job before the locked toolchain installs.
