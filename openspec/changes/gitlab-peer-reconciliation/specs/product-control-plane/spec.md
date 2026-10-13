# Spec Delta

## ADDED Requirements

### Requirement: A returning publication peer receives the exact published objects

When a declared peer returns after missing publications, its catch-up SHALL
publish the same locally signed commits and annotated release tags that the
other peer received, through the native publisher, without rebuilding,
re-signing, or rewriting history. The peer SHALL remain reported incomplete
until its own CI passes for those exact objects.

#### Scenario: A peer missed branch and tag publications

- **WHEN** one peer was unavailable while the other received accepted branches
  and release tags
- **THEN** catch-up SHALL publish those exact branch heads and tag objects to it
- **AND** SHALL NOT rebuild, re-sign, or rewrite any published object.

#### Scenario: The returning peer cannot supply its tools

- **WHEN** the returning peer's locked tool supply is unavailable
- **THEN** its pipeline SHALL fail before any product job
- **AND** the peer SHALL remain incomplete; publishing branches alone SHALL NOT
  claim its recovery.
