## ADDED Requirements

### Requirement: Native verification runs the declared Mise release

Local verification and every Forge job SHALL run the Mise release that the
repository declares. A Mise release whose diagnostics the repository's tests
assert SHALL become the CUE pin and the repository floor together, and a
runner-owned Mise SHALL be replaced before a job consumes that declaration.

#### Scenario: A Mise release changes an asserted diagnostic

- **WHEN** a repository test asserts a diagnostic that only a newer Mise release
  prints
- **THEN** the CUE pin, the generated Forge projections and the repository Mise
  floor SHALL name that release
- **AND** the hosted native jobs SHALL pass with it before integration.

#### Scenario: A runner owns its Mise

- **WHEN** the declared Mise advances while a runner owns its Mise executable,
  such as GitLab's Windows ARM64 runner
- **THEN** that runner's executable and shim SHALL be replaced with the verified
  release before any of its jobs consumes the declaration.
