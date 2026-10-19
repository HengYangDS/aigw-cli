## MODIFIED Requirements

### Requirement: Codex selections respect TOML scope and literal identity

AIGW SHALL use the locked native TOML parser for Codex root model, provider,
and catalogue selections. Projection and withdrawal SHALL preserve unrelated
bytes, including quoted, multiline, dotted, and named-profile keys and arrays
of tables. Replacement values SHALL be literal, never regex substitutions.
Invalid or ambiguous selections SHALL fail before writes. Root ownership SHALL
require the attributed sidecar and recorded semantic values; decorative
comments alone do not grant authority.

#### Scenario: Project and withdraw a literal model selection

- **WHEN** the original root selection uses quoted keys, multiline values, or
  comments and the selected model contains dollar signs, quotes, or backslashes
- **THEN** projection retains the exact selected value
- **AND** withdrawal restores the recorded original selection source
- **AND** named-profile settings remain unchanged through both operations.

#### Scenario: Root and named-profile catalogues coexist

- **WHEN** a named profile has its own catalogue and the selected root model
  needs a proven catalogue projection
- **THEN** AIGW projects only the root reference and its owned catalogue
- **AND** withdrawal removes only those owned outputs.

#### Scenario: A selection cannot be represented safely

- **WHEN** a selection is malformed, ambiguous, invalid UTF-8, or contains an
  inadmissible control character
- **THEN** the operation reports the input error without normalizing it into a
  different model or applying configuration writes.

#### Scenario: The native client removes decorative ownership comments

- **GIVEN** the attributed sidecar, provider values, scheduler values, and
  catalogue still match the last AIGW projection
- **WHEN** the native client rewrites the root provider and model assignments
  without their decorative ownership comments
- **THEN** inspection SHALL accept unchanged semantic values
- **AND** synchronization SHALL restore the canonical comments without changing
  unrelated client configuration.

#### Scenario: A root selection changes outside explicit Route selection

- **WHEN** the root provider or model value no longer matches the attributed
  AIGW projection
- **THEN** repair, and synchronization unless it adopts a model-only change
  under "Synchronization adopts a Codex model choice", SHALL reject the
  conflict before writing configuration, sidecar, catalogue, or credential
  state
- **AND** an explicit Route verification MAY replace the values only in its
  private projection copy
- **AND** an explicit operator Route selection MAY replace the root values only
  after the provider block, scheduler, catalogue, and sidecar still prove the
  existing AIGW projection.
