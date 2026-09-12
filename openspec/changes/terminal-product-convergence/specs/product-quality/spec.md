## MODIFIED Requirements

### Requirement: faithful quantitative quality evidence

Go statement coverage SHALL be measured under one machine policy owning the
aggregate floor, package observation, comparison, risk, remediation, and
review. Every canonical production package MUST remain visible. Packages with
measurable statements MUST execute owned statements and retain exact ratios;
proven declaration-only and native zero-statement packages MUST report not
applicable instead of an invented ratio. Evidence MUST bind raw counts, package,
revision and tree, toolchain, and policy digest. The repository SHALL NOT infer
branch coverage from statement data or depend on an unavailable analyzer to
manufacture a stronger-looking claim.

#### Scenario: quantitative evidence is evaluated

- **WHEN** coverage is admitted for promotion
- **THEN** aggregate statement evidence SHALL be strictly greater than 95
  percent
- **AND** every canonical production package SHALL be present in the same
  complete evidence set
- **AND** every package with measurable statements SHALL remain executed and
  report its exact statement ratio
- **AND** the verdict SHALL be independent of duplicated literals or inferred
  metrics.

#### Scenario: a quantitative boundary or observation contract is not met

- **WHEN** the aggregate ratio is equal to or below the canonical floor, or a
  package with measurable statements is absent or wholly unexecuted, or the
  observation is contradictory or lacks bound raw evidence
- **THEN** local verification, exact-HEAD proof, and hosted CI SHALL fail before
  promotion.

#### Scenario: a package has no statement denominator

- **WHEN** native Go evidence contains zero measured statements, or the package
  has no counters and Go-selected source proves it has no function bodies
- **THEN** the package SHALL remain visible with coverage not applicable
- **AND** it SHALL add neither fabricated statements nor a percentage to the
  aggregate
- **AND** missing, unreadable or mismatched source evidence SHALL fail rather
  than silently exempt a package.

#### Scenario: an unsupported coverage metric is proposed

- **WHEN** the locked language toolchain cannot produce a claimed metric and no
  maintained admitted analyzer owns it
- **THEN** the repository SHALL omit that claim rather than infer, relabel, or
  preserve it through an abandoned dependency.

#### Scenario: statement data is presented as branch evidence

- **WHEN** a result derives a branch claim from a statement-only profile
- **THEN** the evidence SHALL be rejected as semantically invalid.

#### Scenario: a package owns no branches

- **WHEN** a canonical package contains statements but no independently
  measurable branch decisions
- **THEN** the package SHALL remain visible through its exact statement ratio
- **AND** the repository SHALL NOT invent a 100-percent branch ratio.

#### Scenario: aggregate coverage carries the quantitative veto

- **WHEN** a package has a small or volatile denominator while the aggregate
  floor passes
- **THEN** the exact package ratio SHALL remain visible for review
- **AND** the package ratio SHALL NOT independently veto an otherwise valid
  aggregate result.

## ADDED Requirements

### Requirement: Quality coverage has one positive authority

The architecture policy SHALL assign each tracked carrier exactly one semantic
responsibility. The CI entrypoint and native tool configurations SHALL own the
executable checks and their scopes, collectively covering every applicable
formatting, lint, type, semantic, test, security, and generated-projection
concern. Architecture classification alone SHALL NOT establish quality
coverage. Rules SHALL state the required shape positively; exclusions SHALL
be narrow, justified, and owned by the same authority.

#### Scenario: A tracked carrier is added

- **WHEN** a source, test, configuration, documentation, schema, workflow, or
  release file becomes tracked
- **THEN** architecture classification resolves one responsibility and the
  existing native check scopes cover every applicable concern
- **AND** no file is silently uncovered or governed by competing policies.

#### Scenario: Architecture policy is evaluated

- **WHEN** a policy declares carrier responsibilities and selectors
- **THEN** the architecture gate verifies unique ownership and dependency rules
- **AND** acceptance of check execution requires the separately executed native
  gates, not a duplicate list of check names in that policy.

#### Scenario: A generic checker exists

- **WHEN** a maintained formatter, linter, type, security, dependency, or
  documentation tool covers a required generic concern
- **THEN** the repository uses that tool rather than a custom duplicate
- **AND** retains custom logic only for a documented product invariant.

#### Scenario: A repository npm check executes

- **WHEN** the Go quality gate invokes a formatter, Markdown linter, or OpenSpec
  validator
- **THEN** the locked Node runtime executes its native package script against
  checkout-local dependencies
- **AND** missing dependencies or a nonzero validator exit fail the gate even
  if stdout contains a valid-looking result.

#### Scenario: Native platform source is checked

- **WHEN** native acceptance runs on macOS, Linux, or Windows
- **THEN** the shared Go static-quality policy SHALL execute against that
  host's platform-selected product, tool and test sources before behavioral
  tests and release acceptance
- **AND** a static-check failure SHALL stop those later stages
- **AND** another platform's successful check SHALL NOT substitute for it.

#### Scenario: Quality or release configuration is invalid

- **WHEN** the shared quality command reads golangci-lint or GoReleaser
  configuration
- **THEN** the owning tool's native configuration validator SHALL run before
  source analysis or artifact construction
- **AND** unknown fields, missing input, or a failing validator SHALL fail the
  shared gate even when the file is syntactically valid YAML
- **AND** validation SHALL leave configuration bytes unchanged.

#### Scenario: Markdown configuration is validated before linting

- **WHEN** the shared quality command reads the repository Markdown policy
- **THEN** it SHALL validate options against the CLI schema shipped with the
  locked Markdownlint package and base and override rules against the shipped
  strict built-in-rule schema
- **AND** unknown options or rules, invalid parameter values, multiple YAML
  documents and missing or malformed schema inputs SHALL fail the gate
- **AND** schema references SHALL resolve only from the explicit local inputs,
  without network fetching, copied rule tables or input mutation.

#### Scenario: Current source is scanned for secrets

- **WHEN** the source gate checks credentials in the current checkout
- **THEN** native Gitleaks SHALL scan current regular tracked files, including
  ignored tracked paths, and nonignored untracked files through the existing
  Git inventory
- **AND** path-specific policy and redacted findings SHALL remain effective
- **AND** deleted paths, symlink targets and ignored untracked output SHALL
  remain outside that scope; current-file evidence SHALL NOT imply a history
  scan
- **AND** the scan SHALL preserve source bytes, propagate native failures, and
  reclaim its private input projection after success or failure.

#### Scenario: Markdown sections remain navigable

- **WHEN** current Markdown uses decorative heading punctuation or standalone
  emphasis instead of a section heading
- **THEN** the native Markdown gate SHALL reject the structural defect
- **AND** question headings, emphasized sentences and official OpenSpec
  Goals/Non-Goals labels SHALL remain valid
- **AND** table-column alignment SHALL be checked by the native rule rather
  than a repository-specific parser.

### Requirement: Quantitative policy is evidence-derived

Complexity, executable lines, nesting, parameters, coverage, performance, and
test-size thresholds SHALL derive from the risk they protect and the observed
repository distribution. A threshold SHALL include its scope, rationale,
comparison semantics, review condition, and remediation path.

#### Scenario: A threshold changes

- **WHEN** maintainers tighten or relax a quantitative limit
- **THEN** the change records measured evidence and the protected failure mode
- **AND** does not treat a lower number as intrinsically better.

#### Scenario: Multiple rules report one declaration

- **WHEN** independent native rules find violations at the same source line
- **THEN** the Go quality command SHALL retain each rule's diagnostic
- **AND** a combined trial SHALL NOT count a line-deduplicated report as a
  complete per-rule inventory.

#### Scenario: Package evidence cannot be written

- **WHEN** coverage reporting cannot write a measured, zero-statement or
  declaration-only package observation
- **THEN** the gate SHALL fail and preserve the output error
- **AND** it SHALL reclaim its temporary coverage profile without reporting
  successful aggregate acceptance.

### Requirement: Warnings are owned failures

Every repository-owned warning emitted by a supported build, test, analysis,
documentation, packaging, or CI path SHALL be resolved at its semantic owner.

#### Scenario: A supported gate emits a warning

- **WHEN** the warning is attributable to repository source or configuration
- **THEN** the gate fails until the cause is removed
- **AND** a blanket filter, baseline, or ignored exit code is not accepted as
  the repair.
