## ADDED Requirements

### Requirement: A proof runs each test once and in parallel where it can

Each test SHALL run once in a full proof: a gate that selects tests by name SHALL match only the tests of its own build tag. A test of the acceptance package SHALL run in parallel unless it changes process state, and a structural test SHALL keep it so.

#### Scenario: A tagged gate selects its tests

- **WHEN** the quality graph's `selection-matrix` gate selects tests by name
- **THEN** its pattern SHALL match no test of the acceptance package's untagged files

#### Scenario: A new acceptance test is added

- **WHEN** a top-level test is added to the acceptance package without calling `t.Parallel` first
- **THEN** the package's structural test SHALL fail unless the new test calls `t.Setenv`
