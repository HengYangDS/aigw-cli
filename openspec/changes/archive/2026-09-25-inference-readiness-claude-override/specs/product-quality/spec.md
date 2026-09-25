## MODIFIED Requirements

### Requirement: one complete quality graph

One quality graph SHALL serve local development, exact-HEAD proof, GitLab, and
GitHub. A declarative topology generates Forge files; repository commands own
behavior and projection drift SHALL fail first. Product targets, artifacts,
native acceptance, and host compatibility SHALL remain distinct claims. Each
admitted event SHALL receive its required graph; cross-event reuse requires an
explicit verifier, and cross-compilation SHALL prove artifacts only.

#### Scenario: a new repository owner is added

- **WHEN** tracked material is added or changed
- **THEN** its semantic class SHALL select every applicable quality check without an exclusion.

#### Scenario: a new package or test owner is added

- **WHEN** tracked Go source changes
- **THEN** architecture, static analysis, formatting, coverage, governance, and cross-platform contracts SHALL evaluate the new owner without an exclusion list.

#### Scenario: a projection diverges

- **WHEN** a tracked Forge file differs from the deterministic projection
- **THEN** source verification SHALL fail with the exact file before expensive tests.

#### Scenario: A required native runner is unavailable

- **WHEN** a selected Forge lacks an admitted executor for a supported platform
- **THEN** its required job SHALL remain in the projection and that peer SHALL
  not be accepted as green
- **AND** another peer's result, operating system, or cross-compile SHALL NOT
  substitute for that peer-local native job.

#### Scenario: a release asset is cross-compiled

- **WHEN** CI produces an archive for an OS and architecture not represented by that runner
- **THEN** the archive MAY satisfy the release-asset matrix
- **AND** it SHALL NOT be reported as native acceptance or developer-host proof.

#### Scenario: a projection is regenerated

- **WHEN** the CI authority is rendered
- **THEN** every Forge-native file SHALL be produced deterministically
- **AND** no separate parser or duplicated policy SHALL be required.

#### Scenario: A GitLab job uses a toolchain container

- **WHEN** the runner prepares a container-backed verification job
- **THEN** the projected image configuration yields control to the runner shell
- **AND** the image's own entrypoint cannot reinterpret runner shell arguments

#### Scenario: Tests run inside a Forge job

- **WHEN** a test verifies the generic source gate sequence
- **THEN** it is independent of inherited Forge provenance variables
- **AND** dedicated provenance tests supply their own complete inputs

#### Scenario: A required native runner is misconfigured

- **WHEN** its operating-system shell or locked toolchain cannot start
- **THEN** the native gate fails explicitly
- **AND** no cross-build or different operating system is reported as a substitute

#### Scenario: a developer submits a proposal for review

- **WHEN** a proposal targets `dev` through a pull request or merge request
- **THEN** the review head SHA SHALL receive the complete verification graph
- **AND** the proposal branch push SHALL NOT start a parallel copy of that graph.

#### Scenario: a maintainer publishes an accepted product object

- **WHEN** local accepted `main` is projected unchanged to peer `main` and `dev`
- **THEN** the peer SHALL verify each resulting accepted-branch and release-branch
  event for its exact object
- **AND** only the release-branch push SHALL require accepted-ref parity; equal
  object IDs alone SHALL NOT suppress the accepted-branch graph.

#### Scenario: explicit diagnosis is required

- **WHEN** a maintainer explicitly dispatches verification without a platform selector
- **THEN** the selected Forge SHALL run the complete graph for the selected ref
- **AND** a targeted platform dispatch SHALL remain diagnostic rather than
  complete review or release evidence.

### Requirement: Warnings are owned failures

Every repository-owned warning and nonempty native validation finding emitted
by a supported build, test, analysis, documentation, packaging, or CI path
SHALL be resolved at its semantic owner. A validator's advisory severity does
not waive this repository's clean-evidence policy.

#### Scenario: A supported gate emits a warning

- **WHEN** the warning is attributable to repository source or configuration
- **THEN** the gate fails until the cause is removed
- **AND** a blanket filter, baseline, or ignored exit code is not accepted as
  the repair.

#### Scenario: Native validation distinguishes advice from failure

- **WHEN** OpenSpec reports `INFO`, `WARNING`, `ERROR`, or a failed summary
- **THEN** the repository gate fails and reports every finding without filtering
  it, even if the native validator classifies the item as informational
- **AND** unknown severity, malformed evidence, or diagnostic output failure
  also fails admission.

## REMOVED Requirements

### Requirement: Forge capability projection

**Reason:** A separate Forge-capacity list let one peer silently omit a required
native platform and duplicated the native matrix owned by the CI graph.

**Migration:** The complete peer-local native matrix and manual-selection
behavior are owned by `ci-diagnostics:One CI graph projects to independent
Forges`; product-level quality remains in `one complete quality graph`.
