## ADDED Requirements

### Requirement: A proof runs each test once and in parallel where it can

Each test SHALL run once in a full proof: a gate that selects tests by name SHALL match only the tests of its own build tag. A test of the acceptance package SHALL run in parallel unless it changes process state, and the repository's lint SHALL keep it so.

#### Scenario: A tagged gate selects its tests

- **WHEN** the quality graph's `selection-matrix` gate selects tests by name
- **THEN** its pattern SHALL match no test of the acceptance package's untagged files

#### Scenario: A new acceptance test is added

- **WHEN** a top-level test is added to the acceptance package without calling `t.Parallel`
- **THEN** the repository's lint SHALL refuse it unless the test calls `t.Setenv` or `t.Chdir`, or names its reason in a
  `nolint:paralleltest` directive
