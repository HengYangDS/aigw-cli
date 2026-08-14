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

### Requirement: complete delivery evidence

Quality completion SHALL require distinct evidence for the complete local
graph, exact-HEAD proof, native hosted CI, independent Forge publication, asset
integrity, installation, runtime acceptance, and repository housekeeping. A
release SHALL be complete only when its signed tag, immutable assets, checksums,
Forge-native release record, and supported-platform acceptance are verified
independently on each declared publication plane.

#### Scenario: local proof passes but delivery is incomplete

- **WHEN** hosted CI, a publication plane, asset parity, installation, runtime acceptance, or lane retirement remains unverified
- **THEN** the repository SHALL report that stage as incomplete and SHALL NOT claim terminal completion.

#### Scenario: terminal closeout succeeds

- **WHEN** every delivery stage passes for the exact accepted head and obsolete lanes, policies, compatibility paths, temporary assets, and stale runtime residue are retired
- **THEN** the repository MAY report completion with receipts for each independent boundary.

#### Scenario: release metadata exists without publication

- **WHEN** VERSION and CHANGELOG name a release but either Forge lacks its signed tag, release record, or assets
- **THEN** delivery SHALL remain incomplete.

#### Scenario: both publication planes complete

- **WHEN** GitLab and GitHub independently verify and publish the same source tree
- **THEN** their commit and tag object identifiers MAY differ
- **AND** their source tree, version, asset manifest, and supported-platform semantics SHALL agree.
