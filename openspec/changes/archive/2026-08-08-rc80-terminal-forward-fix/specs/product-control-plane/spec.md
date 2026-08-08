## ADDED Requirements

### Requirement: Portable source and user contract

AIGW SHALL build and verify from its own repository and SHALL document every
public setup input without requiring another repository, a workstation-local
path, or an undocumented environment variable.

#### Scenario: An operator selects the environment secret backend

- **WHEN** `AIGW_SECRET_BACKEND=env` is selected
- **THEN** AIGW reads `AIGW_TOKEN_<ACCOUNT>` slots without writing them
- **AND** the README explains the behavior without exposing a real token.

#### Scenario: A contributor verifies rc.80 on a supported host

- **WHEN** native Linux, Windows, or macOS verification runs
- **THEN** repository-controlled fixtures exercise equivalent product meaning
- **AND** every package and the aggregate remain strictly above 95% coverage.
