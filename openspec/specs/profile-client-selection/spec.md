# profile-client-selection Specification

## Purpose

TBD - created by archiving change codex-profile-client-inference. Update Purpose after archive.

## Requirements

### Requirement: An explicit client-scoped profile is self-describing

When an operator selects a profile without selecting a client, the CLI SHALL
use the profile's declared client as the operation target.

#### Scenario: Connectivity test selects a Codex profile

- **WHEN** the operator runs `aigw test --profile <profile>`
- **AND** the profile declares `client = "codex"`
- **THEN** only the Codex endpoint is tested
- **AND** no redundant `--for codex` input is required

#### Scenario: Live verification selects a Claude Code profile

- **WHEN** the operator runs `aigw verify --profile <profile>`
- **AND** the profile declares `client = "claude"`
- **THEN** one Claude Code protocol request is verified
- **AND** no redundant `--for claude` input is required

### Requirement: Ambiguous or conflicting selection fails closed

Client selection SHALL use only the canonical profile client declaration and
SHALL NOT infer from names, models, endpoints, routes, or providers.

#### Scenario: Selected profile has no declared client

- **WHEN** `--profile` is supplied without `--for`
- **AND** the profile has no declared client
- **THEN** the command fails before a network request
- **AND** the error instructs the operator to provide `--for`

#### Scenario: Explicit client conflicts with the profile

- **WHEN** `--for` and `--profile` select different clients
- **THEN** the command rejects the conflict
- **AND** neither input is silently overridden

### Requirement: Unselected-profile behavior remains stable

Commands SHALL preserve their established selection behavior when the operator
does not name a profile.

#### Scenario: Connectivity test has no explicit profile or client

- **WHEN** the operator runs `aigw test` without `--profile` or `--for`
- **THEN** the command tests the admitted configured clients as before

#### Scenario: Live verification has no explicit profile or client

- **WHEN** the operator runs `aigw verify` without `--profile` or `--for`
- **THEN** the command still requires an explicit verification target
