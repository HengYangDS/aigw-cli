## ADDED Requirements

### Requirement: Quality coverage has one positive authority

The repository SHALL maintain one comprehensible responsibility map assigning
every tracked carrier to its formatter, linter, type checker, semantic checker,
test scope, security check, and generated-projection owner. Rules SHALL state
the required shape positively; exclusions SHALL be narrow, justified, and
owned by the same authority.

#### Scenario: A tracked carrier is added

- **WHEN** a source, test, configuration, documentation, schema, workflow, or
  release file becomes tracked
- **THEN** the quality graph assigns its applicable checks
- **AND** no file is silently uncovered or governed by competing policies.

#### Scenario: A generic checker exists

- **WHEN** a maintained formatter, linter, type, security, dependency, or
  documentation tool covers a required generic concern
- **THEN** the repository uses that tool rather than a custom duplicate
- **AND** retains custom logic only for a documented product invariant.

### Requirement: Quantitative policy is evidence-derived

Complexity, executable lines, nesting, parameters, coverage, performance, and
test-size thresholds SHALL derive from the risk they protect and the observed
repository distribution. A threshold SHALL include its scope, rationale,
comparison semantics, review condition, and remediation path.

#### Scenario: A threshold changes

- **WHEN** maintainers tighten or relax a quantitative limit
- **THEN** the change records measured evidence and the protected failure mode
- **AND** does not treat a lower number as intrinsically better.

### Requirement: Warnings are owned failures

Every repository-owned warning emitted by a supported build, test, analysis,
documentation, packaging, or CI path SHALL be resolved at its semantic owner.

#### Scenario: A supported gate emits a warning

- **WHEN** the warning is attributable to repository source or configuration
- **THEN** the gate fails until the cause is removed
- **AND** a blanket filter, baseline, or ignored exit code is not accepted as
  the repair.
