## ADDED Requirements

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

## REMOVED Requirements

### Requirement: Previous local configuration migrates once

**Reason:** The previous default-plus-overrides schema is no longer a supported
runtime input. Retaining its decoder preserves a second Route authority and can
silently reinterpret an obsolete global selection as current user intent.

**Migration:** Replace an earlier configuration explicitly with the reviewed
current schema. AIGW reports the encountered and required versions and does not
guess per-client Routes.

#### Scenario: Read an earlier local schema

- **WHEN** AIGW reads a configuration whose version is not current
- **THEN** it reports the encountered and required versions
- **AND** performs no migration, persistence, credential access, or projection.
