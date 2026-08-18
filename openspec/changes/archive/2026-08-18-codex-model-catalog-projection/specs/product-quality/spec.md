## MODIFIED Requirements

### Requirement: faithful quantitative quality evidence

The repository SHALL measure statement and branch coverage independently. The
canonical machine policy SHALL own the aggregate floor, package-observation
contract, comparison semantics, risk model, false-positive cost, remediation
path, and review condition. Every production package SHALL appear in the
evidence, execute its owned statements and branches, and retain exact diagnostic
ratios. Branchless packages SHALL remain visible and report 100-percent branch
coverage. Evidence SHALL retain raw counts, package identity, source revision
and tree, analyzer identity, and policy digest. No prose, CI projection, or
tool-native formatting file SHALL own a competing threshold.

#### Scenario: quantitative evidence is evaluated

- **WHEN** coverage is admitted for promotion
- **THEN** aggregate statement and branch evidence SHALL each be strictly greater than 95 percent
- **AND** every canonical production package SHALL be present in the same complete evidence set
- **AND** every package SHALL remain executed and report exact statement and branch ratios
- **AND** the verdict SHALL be independent of duplicated literals or inferred metrics.

#### Scenario: a quantitative boundary or observation contract is not met

- **WHEN** an aggregate ratio is equal to or below the canonical floor, or a package is absent, wholly unexecuted, duplicated, or lacks bound raw evidence
- **THEN** local verification, exact-HEAD proof, and hosted CI SHALL fail before promotion.

#### Scenario: statement data is presented as branch evidence

- **WHEN** a result derives a branch claim from a statement-only profile
- **THEN** the evidence SHALL be rejected as semantically invalid.

#### Scenario: a package owns no branches

- **WHEN** the branch analyzer reports a present canonical package with zero branch decisions
- **THEN** that package SHALL remain visible with a 100-percent branch ratio
- **AND** it SHALL NOT be treated as absent or unexecuted.

#### Scenario: aggregate coverage carries the quantitative veto

- **WHEN** a package has a small or volatile denominator while the aggregate floor passes
- **THEN** the exact package ratio SHALL remain visible for review
- **AND** the package ratio SHALL NOT independently veto an otherwise valid aggregate result.
