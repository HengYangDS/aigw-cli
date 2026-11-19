# quantitative-quality Specification

## Purpose

Define how AIGW measures its quality in numbers: what a measurement must be faithful to, which authority
says what coverage counts, where a threshold comes from and how one coverage attempt is kept, without
turning repository-specific measurements into arbitrary merge vetoes.

## Requirements

### Requirement: faithful quantitative quality evidence

One machine policy SHALL own Go statement coverage, including the aggregate
floor, package observation, comparison, risk, remediation, and review. Every
production package MUST remain visible; measurable packages MUST execute owned
statements with exact ratios, while proven zero-statement packages MUST report
not applicable. Evidence MUST bind raw counts, package, revision, tree,
toolchain, and policy. Statement data SHALL NOT be relabeled as branch coverage.

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

### Requirement: Quality coverage has one positive authority

Architecture policy SHALL assign each tracked carrier one semantic owner.
The CI entrypoint and native tool configurations SHALL own executable scopes
covering every applicable format, lint, type, semantic, test, security, and
projection concern. Classification alone SHALL NOT prove execution. Rules SHALL
state positive required shape; exclusions SHALL remain narrow, justified, and
owned by the same authority.

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

#### Scenario: The complete toolchain is qualified on a native platform

- **WHEN** an operator explicitly selects full native quality
- **THEN** the native entrypoint SHALL execute the existing complete quality
  graph before platform-selected behavioral tests and packaged acceptance
- **AND** each quality command SHALL run once, without a second policy list
- **AND** any failed command SHALL stop all later commands
- **AND** ordinary native acceptance SHALL retain its smaller default path
- **AND** source-signature admission SHALL remain a distinct publication check.

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

#### Scenario: The governing framework tightens a shared size limit

- **WHEN** ETHOS, which governs this repository, lowers a per-file size limit
  that the repository's own gate also enforces
- **THEN** the repository's limit SHALL follow it in the same change, so that no
  file passes one gate and fails the other
- **AND** each file above the new limit SHALL give up a whole concern to a file
  named after it rather than being compressed.

### Requirement: Native Go events retain one coverage attempt

The coverage command SHALL optionally emit original Go test JSON on stdout and summaries on stderr, and
write JUnit XML and LCOV reports. Events, retained counters and reports SHALL come from one native
invocation with the selected inventory, tags, race option, atomic instrumentation and threshold policy
unchanged; no other exact-HEAD proof gate SHALL run that attempt's tests again.

#### Scenario: Machine-readable coverage succeeds

- **WHEN** coverage selects `--json` and an explicit profile destination
- **THEN** stdout SHALL contain only the native test event bytes
- **AND** package and aggregate summaries SHALL remain separately observable
- **AND** the retained profile SHALL contain the same attempt's counters.

#### Scenario: The native test attempt fails

- **WHEN** the selected native tests fail after emitting events and a profile
- **THEN** their event bytes and retained diagnostic counters SHALL remain observable
- **AND** the command SHALL fail without qualifying partial coverage
- **AND** temporary profile cleanup SHALL remain required.

#### Scenario: Test and line reports are requested

- **WHEN** coverage selects a JUnit destination and an LCOV destination
- **THEN** the JUnit report SHALL hold one test case for each test result of that attempt, with its failure or
  skip
- **AND** a package that fails without a failing test SHALL appear as a test case with an error
- **AND** the LCOV report SHALL hold one record for each selected production source file by repository path,
  with the lines of that attempt's statement blocks and their counts, and no lines for a file in which the
  attempt's build instrumented no statement
- **AND** the LCOV report SHALL hold no branch or function records
- **AND** a profile file outside the selected inventory SHALL fail the command
- **AND** a reader SHALL never observe a partially written report.

#### Scenario: A test of a failed package reports no result

- **WHEN** coverage writes a JUnit report without `--json`, and a package fails while one of its tests has
  output but no result, as a timeout or a panic during that test leaves it
- **THEN** the command's output SHALL show that test's output
- **AND** the JUnit report SHALL hold that test as a test case with an error and that output.

#### Scenario: The exact-HEAD proof judges Go behavior

- **WHEN** the exact-HEAD proof evaluates the behavior of the Go sources
- **THEN** it SHALL read the reports written by the coverage gate's own attempt
- **AND** no other proof gate SHALL run that attempt's tests a second time.
