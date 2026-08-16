## MODIFIED Requirements

### Requirement: faithful quantitative quality evidence

The repository SHALL measure statement and branch coverage independently. The
canonical machine policy SHALL own the floor, comparison, exact package and
aggregate scope, risk model, false-positive cost, remediation path, and review
condition. Evidence SHALL retain raw counts, package identity, source revision
and tree, analyzer identity, and policy digest. No prose, CI projection, or
tool-native formatting file SHALL own a competing threshold.

#### Scenario: quantitative evidence is evaluated

- **WHEN** package or aggregate coverage is admitted for promotion
- **THEN** the exact raw evidence satisfies the canonical machine policy
- **AND** the verdict is independent of duplicated literals or inferred metrics.

#### Scenario: any quantitative boundary is not met

- **WHEN** a package or aggregate misses the canonical policy, is absent, is duplicated, or lacks bound raw evidence
- **THEN** local verification, exact-HEAD proof, and hosted CI SHALL fail before promotion.

#### Scenario: statement data is presented as branch evidence

- **WHEN** a result derives a branch claim from a statement-only profile
- **THEN** the evidence SHALL be rejected as semantically invalid.

#### Scenario: the floor creates repeated false positives

- **WHEN** legitimate changes are repeatedly blocked solely by denominator granularity
- **THEN** maintainers review the canonical policy against its recorded risk model and cost
- **AND** no package exclusion or local override is introduced.

### Requirement: portable repository text

Tracked text SHALL use deterministic encoding and line-ending semantics across
supported hosts. The repository-wide checker SHALL enforce byte invariants that
affect portability or diff integrity. Blank-line quantity and configuration
table spacing SHALL remain presentation concerns unless a separate product-risk
model is admitted.

#### Scenario: text contains a byte-level defect

- **WHEN** tracked text contains CR line endings, trailing whitespace, or lacks a final newline
- **THEN** repository quality SHALL report the exact file and line.

#### Scenario: a contributor uses another supported host

- **WHEN** the repository is cloned under another operating system, user, directory, or Forge
- **THEN** checkout, verification, build, installation, and release contracts SHALL remain discoverable and executable from repository-owned inputs.

#### Scenario: a contributor uses another operating system

- **WHEN** Git checks out tracked text
- **THEN** line endings and executable semantics SHALL remain deterministic.

#### Scenario: Native Windows renders the CI projection

- **WHEN** the repository and a test fixture reside on different Windows volumes
- **THEN** CUE evaluates the CI authority from the selected repository root
- **AND** generated Forge paths resolve within that root
- **AND** the same focused contracts pass on macOS, Linux, and Windows.

#### Scenario: valid text uses a different readable spacing style

- **WHEN** Markdown or configuration uses semantically valid blank-line spacing
- **THEN** the repository-wide byte checker SHALL not reject it
- **AND** formatters, serializers, and review retain their own scoped authority.
