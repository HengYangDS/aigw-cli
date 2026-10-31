## MODIFIED Requirements

### Requirement: Deterministic local verification

Local verification MUST use controlled fixtures rather than an undeclared
public network dependency or host-provided Git templates and hooks, and it
SHALL leave no temporary directory of its own behind. A native qualification
that needs public services SHALL run only in a quality gate that declares
network access.

#### Scenario: A local test depends on external state

- **WHEN** a verification test would contact a public endpoint instead of its
  controlled fixture
- **THEN** local verification SHALL fail or the fixture SHALL intercept the
  exact request without public network I/O

#### Scenario: A host provides Git templates

- **WHEN** the host configures a Git template directory with hooks,
  configuration or exclusions
- **THEN** repositories created by verification tests SHALL contain none of
  that content

#### Scenario: A host provides global Git hooks

- **WHEN** the host's global Git configuration names a hook directory
- **THEN** verification tests SHALL NOT execute those hooks
- **AND** fixtures SHALL keep their own hook directories and the host's other
  global settings

#### Scenario: A native tool needs public services

- **WHEN** a test qualifies a native tool against public advisory or license
  services
- **THEN** it SHALL run only in a quality gate that declares network access
- **AND** offline behavior tests SHALL cover the same evidence handling with
  controlled fixtures

#### Scenario: A test runs its own binary as a helper process

- **WHEN** a test runs its test binary again as a helper process with the
  test's environment, and the helper exits inside a test
- **THEN** the helper SHALL apply the same host Git policy as the test that
  started it
- **AND** after a complete local verification the temporary directory SHALL
  hold no directory that the verification created for its own run, although a
  tool's persistent cache MAY remain
