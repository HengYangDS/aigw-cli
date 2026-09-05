## ADDED Requirements

### Requirement: Generated projections have one canonical model

Generated client configuration, CI, release metadata, specifications, and other
reviewed text SHALL be deterministic projections of one named canonical model.
Repository validation SHALL regenerate or compare the projection and reject
semantic or byte-level drift.

#### Scenario: A generated projection is reviewed

- **WHEN** a projection differs from its canonical model
- **THEN** validation identifies the owning model and projection
- **AND** the projection is regenerated rather than independently patched.

#### Scenario: Equivalent output is rendered twice

- **WHEN** unchanged canonical input is rendered in the same supported toolchain
- **THEN** both outputs are byte-identical, end with one newline, contain no
  host path or secret, and use a stable readable ordering.
