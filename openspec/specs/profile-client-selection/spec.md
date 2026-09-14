# profile-client-selection Specification

## Purpose

Define deterministic client selection from explicit operator input and the
selected Profile's declared client, without inference from incidental names or
provider details.

## Requirements

### Requirement: An explicit client-scoped profile is self-describing

Every Profile SHALL declare exactly one admitted client and exactly one model.
Commands that receive a Profile SHALL derive the target client from that
Profile rather than require or accept a second client selection. Selecting a
Profile SHALL converge the selected client's available adapter and projection
in the same transaction without enabling or changing another client.

#### Scenario: Connectivity test selects a Codex profile

- **WHEN** the operator runs `aigw test --profile <profile>`
- **AND** the Profile declares `client = "codex"`
- **THEN** only the Codex endpoint is tested
- **AND** no redundant client input is required.

#### Scenario: Live verification selects a Claude Code profile

- **WHEN** the operator runs `aigw verify --profile <profile>`
- **AND** the Profile declares `client = "claude"`
- **THEN** AIGW invokes the synchronized native Claude Code client for one
  bounded verification request and requires its successful final response
- **AND** no redundant client input is required
- **AND** an HTTP endpoint probe alone does not establish this client result.

#### Scenario: Select a Profile

- **WHEN** the operator runs `aigw use <profile>`
- **THEN** AIGW SHALL update only the Route for the Profile's declared client
- **AND** SHALL converge that client's available adapter and projection
- **AND** SHALL leave every other client Route and adapter unchanged.

#### Scenario: Test or verify a named Profile

- **WHEN** the operator supplies `--profile <profile>` to a connectivity or
  live-verification command
- **THEN** AIGW SHALL target only the Profile's declared client
- **AND** SHALL NOT require a redundant client argument.

### Requirement: Ambiguous or conflicting selection fails closed

Client selection SHALL have exactly one authority per invocation. A named
Profile SHALL supply its declared client; otherwise an explicit client SHALL
select that client's current Route. AIGW SHALL NOT infer either value from
names, models, endpoints, providers, or the other client's Route.

#### Scenario: Profile and client selectors are combined

- **WHEN** an operator supplies both `--profile` and `--for`
- **THEN** the command SHALL fail before credential or network access
- **AND** SHALL instruct the operator to keep exactly one selector.

#### Scenario: Selected profile has no declared client

- **WHEN** `--profile` selects an invalid Profile without an admitted client
- **THEN** the command SHALL fail before credential or network access
- **AND** SHALL report that the Profile lacks its required client declaration.

#### Scenario: Explicit client conflicts with the profile

- **WHEN** an operator supplies both `--for` and `--profile`, whether or not
  their client values would match
- **THEN** the command SHALL reject the redundant selectors
- **AND** SHALL instruct the operator to keep exactly one selector.

#### Scenario: Explicit client uses its selected Route

- **WHEN** an operator supplies `--for <client>` without `--profile`
- **THEN** the command SHALL use only that client's selected Route
- **AND** SHALL report the exact missing Route when none is selected.

### Requirement: Unselected-profile behavior remains stable

`aigw test` without a selector SHALL inspect only clients with an explicitly
selected Route. It SHALL NOT select an unselected catalogue Profile or start
a client. `aigw verify` SHALL require an explicit client or Profile before
configuration, credential, or network access; bulk verification SHALL use the
enabled-client scope defined by cli-readiness.

#### Scenario: Connectivity test has no explicit profile or client

- **WHEN** the operator runs `aigw test` without `--profile` or `--for`
- **THEN** the command tests only the admitted clients with a selected Route
- **AND** unselected clients and catalogue Profiles remain outside its scope.

#### Scenario: Live verification has no explicit profile or client

- **WHEN** the operator runs `aigw verify` without `--profile` or `--for`
- **THEN** the command rejects the missing target before reading configuration
- **AND** it requests either `--for <client>` or `--profile <profile>` without
  starting a client, reading a Token, or making a network request.

### Requirement: Route selection has one authority

The configuration SHALL contain one explicit mapping from each selected client
to its Profile. AIGW SHALL NOT resolve a client through a global default,
inheritance, cross-client fallback, or a second override layer.

#### Scenario: Select independent Claude and Codex Profiles

- **WHEN** the operator selects one Claude Profile and one Codex Profile
- **THEN** each client SHALL resolve through its own Route
- **AND** changing either Route SHALL not change the other.

#### Scenario: A client has no selected Route

- **WHEN** an operation requires a client for which no Route is selected
- **THEN** AIGW SHALL report that exact missing Route
- **AND** SHALL recommend selecting a compatible Profile
- **AND** SHALL NOT substitute another client's Profile.

### Requirement: Selection changes exactly one client Route

`aigw use <profile>` SHALL derive one admitted client from the Profile and
transactionally update only that client's Route and owned projection. AIGW
SHALL expose no persistent global default and no bulk-selection state that is
required for readiness.

#### Scenario: Select a Codex Profile

- **WHEN** an operator selects a valid Codex Profile
- **THEN** only the Codex Route and Codex-owned projection may change
- **AND** the Claude Route, credential, and projection remain byte-identical.

#### Scenario: Select a Claude Profile

- **WHEN** an operator selects a valid Claude Profile
- **THEN** only the Claude Route and Claude-owned projection may change
- **AND** the Codex Route, credential, and projection remain byte-identical.

#### Scenario: The unselected client has an external configuration conflict

- **WHEN** an operator selects a valid Profile while another client's projection
  contains external changes
- **THEN** selection preflights and updates only the Profile's declared client
- **AND** the unrelated conflict neither blocks selection nor changes that
  client's configuration or ownership sidecar.

#### Scenario: Re-select the active Profile

- **WHEN** the selected Profile and owned projection already match the desired
  state and any required Account Token is available
- **THEN** the operation succeeds as an observable no-op
- **AND** does not rewrite files, credentials, or verification checkpoints.

#### Scenario: Re-select with a missing Account Token

- **WHEN** the selected Profile is unchanged and interactive selection acquires
  its missing Account Token
- **THEN** the result reports Token storage rather than an unchanged credential
- **AND** unchanged configuration, client files, and checkpoints are not rewritten.

### Requirement: Selection owns its credential transaction

Selection SHALL validate with the command context and reuse the credential
replacement owner's guarded compensation. Before selection commits, failure
or cancellation SHALL compensate its Token and automatic backend choice.
Compensation SHALL preserve any newer state and report incomplete recovery
alongside the original failure. Result rendering is outside this transaction.

#### Scenario: Selection is cancelled

- **WHEN** cancellation occurs before or during validation, or before selection
  commits
- **THEN** the command returns cancellation and applies guarded compensation to
  any Token written by this attempt
- **AND** reports any compensation failure without concealing the cancellation.

#### Scenario: Selection fails after Token storage

- **WHEN** client convergence or configuration persistence fails
- **THEN** unchanged credential postimages and the newly persisted automatic
  backend choice are compensated
- **AND** any compensation error remains observable with the original failure.

#### Scenario: Result output fails after selection commits

- **WHEN** selection commits but its result cannot be written
- **THEN** the output error is returned without undoing committed credentials.
