## MODIFIED Requirements

### Requirement: one complete quality graph

The repository SHALL expose one executable quality graph reused without policy
duplication by local development, exact-HEAD governance proof, GitLab CI, and
GitHub Actions. The graph SHALL positively classify and verify applicable
formatting, vetting, static analysis, architecture, security, dependencies,
documentation links, tests, product source, repository tools, CI, OpenSpec,
build, release, installation, runtime acceptance, and native macOS, Linux, and
Windows material. Each policy and behavior SHALL have one semantic owner;
projections SHALL invoke that owner rather than restate it. Warnings, unavailable
required runners, and skipped required platforms SHALL fail explicitly within a
bounded interval rather than wait indefinitely or be represented as success.

#### Scenario: a new repository owner is added

- **WHEN** tracked material is added or changed
- **THEN** its semantic class SHALL select all applicable architecture, static-analysis, formatting, coverage, governance, portability, documentation, build, release, installation, and runtime-contract checks without adding an exclusion

#### Scenario: a new package or test owner is added

- **WHEN** tracked Go source changes
- **THEN** architecture, static analysis, formatting, coverage, governance, and cross-platform contracts SHALL evaluate the new owner without an exclusion list

#### Scenario: a projection diverges

- **WHEN** local, ETHOS, GitLab, or GitHub configuration omits or restates part of the source graph
- **THEN** repository validation SHALL fail with the divergent projection and owner

#### Scenario: A required native runner is unavailable

- **WHEN** a required macOS, Linux, or Windows native job cannot execute
- **THEN** CI reports an unavailable required gate within a bounded interval
- **AND** no cross-compile result substitutes for the native check.

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
