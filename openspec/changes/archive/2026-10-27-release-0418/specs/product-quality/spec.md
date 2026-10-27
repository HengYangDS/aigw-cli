## ADDED Requirements

### Requirement: Benchmark acceptance fails only on product diagnostics

Benchmark acceptance SHALL judge the diagnostics of the product's own prepare
and measured commands. The benchmark tool's statistical warnings SHALL stay in
the retained evidence and SHALL NOT fail acceptance.

#### Scenario: The benchmark tool warns about its statistics

- **WHEN** Hyperfine prints a statistical warning and the measured commands
  print no diagnostic
- **THEN** acceptance SHALL pass
- **AND** the raw output SHALL retain the warning.

#### Scenario: A measured command prints a diagnostic

- **WHEN** the product's prepare or measured command prints a diagnostic
- **THEN** acceptance SHALL fail before timing.

### Requirement: A dependency scan exit needs a normalized advisory

A dependency scan that exits with findings SHALL fail unless its retained
evidence holds at least one normalized advisory.

#### Scenario: The scanner reports findings that normalization cannot name

- **WHEN** OSV-Scanner exits 1 and normalization finds no advisory
- **THEN** the dependency gate SHALL fail
- **AND** its exit record SHALL mark the scan as failed.

### Requirement: Doctor knows a target on another volume is outside home

When the home directory and a target are absolute paths on different volumes,
doctor SHALL label the target as outside the home directory. A relative or
invalid home directory SHALL leave the target's location unknown.

#### Scenario: A Windows target is on another drive or share

- **WHEN** HOME is on one drive and the Codex target is on another drive or a
  UNC share
- **THEN** doctor SHALL label the target as outside the home directory.

#### Scenario: Volume names differ only in letter case

- **WHEN** both paths name the same drive in different letter case
- **THEN** doctor SHALL treat them as one volume.

### Requirement: Artifact jobs prepare only their peer's download tool

A native artifact job SHALL prepare the Forge CLI of its own peer and not the
other peer's. Source and full-quality jobs MAY prepare both.

#### Scenario: A GitHub retained-store run uses peer tool sources

- **WHEN** a GitHub artifact job runs with `tool_source=peer`
- **THEN** it SHALL NOT request glab from any carrier.

### Requirement: Tests keep Mise state private

A test that runs Mise SHALL direct Mise's state directory into the test's own
workspace, so that trust and tracking records for temporary checkouts never
reach the operator's Mise state.

#### Scenario: A test runs Mise in a temporary checkout

- **WHEN** the test runs Mise in its temporary checkout
- **THEN** Mise's tracking record SHALL be written inside the test workspace.
