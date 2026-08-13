# Product quality delta

## MODIFIED Requirements

### Requirement: one complete quality graph

The repository SHALL expose one executable quality graph reused without policy
duplication by local development, exact-HEAD governance proof, GitLab CI, and
GitHub Actions. The graph SHALL positively classify and verify applicable
product source, tests, repository tools, CI, documentation, OpenSpec, build,
release, installation, and runtime-acceptance material. Each policy and behavior
SHALL have one semantic owner; projections SHALL invoke that owner rather than
restate it.

#### Scenario: a new repository owner is added

- **WHEN** tracked material is added or changed
- **THEN** its semantic class SHALL select all applicable architecture, static-analysis, formatting, coverage, governance, portability, documentation, build, release, installation, and runtime-contract checks without adding an exclusion

#### Scenario: a new package or test owner is added

- **WHEN** tracked Go source changes
- **THEN** architecture, static analysis, formatting, coverage, governance, and cross-platform contracts SHALL evaluate the new owner without an exclusion list

#### Scenario: a projection diverges

- **WHEN** local, ETHOS, GitLab, or GitHub configuration omits or restates part of the source graph
- **THEN** repository validation SHALL fail with the divergent projection and owner

### Requirement: portable repository text

The repository SHALL define editor intent, checkout normalization, quality
execution, build, installation, and release behavior using tracked,
platform-neutral configuration. The contract SHALL run through the same
repository-native command on macOS, Linux, and Windows without requiring a
particular interactive shell, personal path, workstation product, IDE, or
foreign repository.

#### Scenario: a contributor uses another supported host

- **WHEN** the repository is cloned under another operating system, user, directory, or Forge
- **THEN** checkout, verification, build, installation, and release contracts SHALL remain discoverable and executable from repository-owned inputs

#### Scenario: a contributor uses another operating system

- **WHEN** Git checks out tracked text
- **THEN** line endings and executable semantics SHALL remain deterministic

### Requirement: actor-independent contribution policy

The repository SHALL require structured commit messages and trusted signatures
without binding a personal name, email, key, fingerprint, host path, signing
program, or Forge credential. Each Forge's protected context SHALL supply its
own actor and trust input independently.

#### Scenario: an admitted team contributor commits

- **WHEN** a commit enters protected history
- **THEN** its message and signature SHALL satisfy repository policy using the explicit publication context for that Forge

#### Scenario: one Forge is unavailable

- **WHEN** GitLab or GitHub cannot verify or publish
- **THEN** the other Forge's verification and publication SHALL remain independently executable and SHALL NOT claim success for the unavailable plane

## ADDED Requirements

### Requirement: faithful quantitative quality evidence

The repository SHALL measure statement coverage and branch coverage
independently. Each measure SHALL be strictly greater than 95 percent for every
package and for the module aggregate. Package completeness SHALL prove that
every package selected by the canonical module query appears exactly once in
both results. Evidence SHALL retain package identity, raw covered and total
counts, source revision and tree, analyzer identity, and policy digest; one
measure SHALL NOT be inferred from another.

#### Scenario: any quantitative boundary is not met

- **WHEN** a package or aggregate has statement or branch coverage of 95 percent or less, is absent, is duplicated, or lacks bound raw evidence
- **THEN** local verification, exact-HEAD proof, and hosted CI SHALL fail before promotion

#### Scenario: statement data is presented as branch evidence

- **WHEN** a result derives a branch claim from a statement-only profile
- **THEN** the evidence SHALL be rejected as semantically invalid

### Requirement: bounded semantic structure

Production, test, and repository-tool code SHALL follow the declared semantic
topology and dependency direction. Enforced limits SHALL cover file and
directory size, function size, decision complexity, nesting, naming, package
documentation, and import ownership. Composition roots SHALL assemble cohesive
owners; shared behavior SHALL live at the smallest stable semantic owner rather
than in a forwarding wrapper, alias-only package, or copied helper.

#### Scenario: structure exceeds an admitted bound

- **WHEN** production, test, or tool code exceeds a declared structural limit or violates semantic ownership
- **THEN** the architecture gate SHALL fail with the exact owner and measured bound

#### Scenario: an ordinary provider is added

- **WHEN** a provider can be represented by Account, endpoint, Profile, and Route data
- **THEN** it SHALL require no provider-specific command, client projection, installer, service lifecycle, or core dependency branch

### Requirement: complete delivery evidence

Quality completion SHALL require distinct evidence for the complete local graph,
exact-HEAD proof, native hosted CI, independent Forge publication, clean
installation, runtime acceptance, and repository housekeeping. Earlier-stage
evidence SHALL NOT substitute for a later stage, and evidence from one Forge
SHALL NOT establish the state of the other.

#### Scenario: local proof passes but delivery is incomplete

- **WHEN** hosted CI, a publication plane, asset parity, installation, runtime acceptance, or lane retirement remains unverified
- **THEN** the repository SHALL report that stage as incomplete and SHALL NOT claim terminal completion

#### Scenario: terminal closeout succeeds

- **WHEN** every delivery stage passes for the exact accepted head and obsolete lanes, policies, compatibility paths, temporary assets, and stale runtime residue are retired
- **THEN** the repository MAY report completion with the receipts for each independent boundary

## Requirement To Task To Proof

| Requirement | Tasks | Proof |
| --- | --- | --- |
| `product-quality:one complete quality graph` | `1.2` | `repository-source-graph-and-projection-contracts` |
| `product-quality:faithful quantitative quality evidence` | `2.2` | `per-package-and-aggregate-statement-branch-package-reports` |
| `product-quality:bounded semantic structure` | `3.1` | `production-test-and-tool-architecture-report` |
| `product-quality:portable repository text` | `4.1` | `native-macos-linux-windows-source-gates` |
| `product-quality:actor-independent contribution policy` | `4.2` | `explicit-trust-provenance-reports` |
| `product-quality:complete delivery evidence` | `5.2` | `exact-head-ci-release-install-runtime-retirement-receipts` |
