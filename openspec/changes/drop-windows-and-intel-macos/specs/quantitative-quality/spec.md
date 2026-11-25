## MODIFIED Requirements

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

- **WHEN** native acceptance runs on macOS or Linux
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
