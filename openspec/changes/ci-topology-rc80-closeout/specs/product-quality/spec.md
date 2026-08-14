## MODIFIED Requirements

### Requirement: one complete quality graph

The repository SHALL expose one logical quality graph reused by local
development, exact-HEAD governance proof, GitLab CI, and GitHub Actions. One
declarative topology authority SHALL generate every Forge-native CI file.
Generated files SHALL NOT own policy. Executable behavior SHALL remain in
repository-owned commands. Projection drift SHALL fail before other source
gates. Product targets, release assets, native acceptance, and developer-host
compatibility SHALL be distinct claims. Cross-compilation SHALL prove only
asset production and SHALL NOT substitute for native execution evidence.

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

- **WHEN** a required macOS, Linux, or Windows native job cannot execute
- **THEN** CI SHALL report the missing evidence within a bounded interval
- **AND** no other Forge result or cross-compile SHALL substitute for the native check.

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
