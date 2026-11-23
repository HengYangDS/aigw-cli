## MODIFIED Requirements

### Requirement: An outside edit to managed client settings has one recovery action

When AIGW refuses because AIGW-managed settings of any client were changed
outside AIGW, other than a client's own model selection, which
`client-chosen-model` owns, every surface that reports the refusal, including
errors, `check` and `status`,
SHALL give the same next action: undo the outside edit, then run `aigw sync`,
or, before a configuration exists, the refused `aigw setup` again. AIGW SHALL
NOT advise a command that refuses on the same state, overwrite the edit, or
relax the guard.

#### Scenario: Retry budgets were edited outside AIGW

- **WHEN** the operator changed managed Codex retry budgets outside AIGW
- **THEN** `use --dry-run`, `client edit --dry-run`, `sync --dry-run`, `sync`,
  `check` and `status` SHALL report the same next action
- **AND** none of them SHALL advise `aigw check` or `aigw sync` alone
- **AND** client files and owned state SHALL stay unchanged.

#### Scenario: The outside edit is undone

- **WHEN** the operator restores AIGW's values and runs `aigw sync`
- **THEN** synchronization SHALL succeed and `check` SHALL report no drift.

#### Scenario: No configuration exists yet

- **WHEN** `aigw setup` refuses an outside edit in a home without `aigw.toml`
- **THEN** its next action SHALL be to undo the edit and run that `aigw setup`
  again, never `aigw sync`, which refuses without a configuration.

### Requirement: Codex model choices respect TOML scope and literal identity

AIGW SHALL read Codex root model, provider, and catalog selections by their TOML
meaning. Projection and withdrawal SHALL preserve unrelated
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

#### Scenario: Root and named-profile catalogs coexist

- **WHEN** a named profile has its own catalog and the selected root model
  needs a proven catalog projection
- **THEN** AIGW projects only the root reference and its owned catalog
- **AND** withdrawal removes only those owned outputs.

#### Scenario: A selection cannot be represented safely

- **WHEN** a selection is malformed, ambiguous, invalid UTF-8, or contains an
  inadmissible control character
- **THEN** the operation reports the input error without normalizing it into a
  different model or applying configuration writes.

#### Scenario: The native client removes decorative ownership comments

- **GIVEN** the attributed sidecar, provider values, scheduler values, and
  catalog still match the last AIGW projection
- **WHEN** the native client rewrites the root provider and model assignments
  without their decorative ownership comments
- **THEN** inspection SHALL accept unchanged semantic values
- **AND** synchronization SHALL restore the canonical comments without changing
  unrelated client configuration.

#### Scenario: A root selection changes outside an explicit selection

- **WHEN** the root provider or model value no longer matches the attributed
  AIGW projection
- **THEN** a command that writes Codex's files, unless it follows a model-only
  change that `client-chosen-model` admits, SHALL reject the conflict before
  writing configuration, sidecar, catalog, or credential state
- **AND** `aigw check --depth client` MAY replace the values only in its
  private copy
- **AND** an explicit `aigw use` MAY replace the root values only after the
  provider block, scheduler, catalog, and sidecar still prove the existing
  AIGW projection.
