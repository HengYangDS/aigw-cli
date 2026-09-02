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

#### Scenario: Re-select the active Profile

- **WHEN** the selected Profile and owned projection already match the desired
  state
- **THEN** the operation succeeds as an observable no-op
- **AND** does not rewrite files, credentials, or verification checkpoints.

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
