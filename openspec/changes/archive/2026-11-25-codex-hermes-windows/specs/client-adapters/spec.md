## ADDED Requirements

### Requirement: Hermes runs each model at its declared context window

For every model of every provider entry that AIGW owns in Hermes's file, AIGW SHALL write the model's declared
context window as that model's `context_length`, under the entry's `models` as a mapping from the model's ID, in the
order in which AIGW lists the models. AIGW SHALL write no window on a provider entry or in Hermes's own `model`
block. Every reading of the models that an entry lists SHALL accept the mapping and the list of IDs that an earlier
release wrote.

#### Scenario: Every listed model carries its window

- **WHEN** AIGW projects Hermes's file
- **THEN** each provider entry that AIGW owns SHALL list each of its models as its ID with `context_length` set to
  the declared window, in AIGW's order
- **AND** no provider entry and no `model` block SHALL hold a `context_length` that AIGW wrote.

#### Scenario: A file that an earlier release wrote

- **GIVEN** Hermes's file lists an entry's models as a list of IDs, with AIGW's record of the file intact
- **WHEN** AIGW projects the file
- **THEN** it SHALL accept the file as its own and write the mapping.

#### Scenario: A model chosen in Hermes among mapped models

- **GIVEN** the file lists an entry's models as the mapping
- **WHEN** the operator selects another listed model in Hermes and `aigw sync` runs
- **THEN** Hermes's selection SHALL name that model, as it does for a list of IDs.

#### Scenario: The declared window changes

- **WHEN** the declared window of a listed model changes
- **THEN** `aigw sync --dry-run` SHALL plan a write of Hermes's file
- **AND** the command that changed the window SHALL write the new number for an enabled Hermes.

#### Scenario: An entry's models reordered outside AIGW

- **WHEN** the models of a provider entry that AIGW owns stand in Hermes's file in an order other than the one that
  AIGW wrote, the order of the configuration before the command, and nothing else of the entry changed
- **THEN** synchronization, a command that projects Hermes and its inspection SHALL refuse it as an outside edit
  without changing configuration or client files
- **AND** a command whose configuration changes the order of a provider's models SHALL write them in the new order.

## MODIFIED Requirements

### Requirement: A record that the previous release reads keeps its shape

A record of a client's file that the previous published release reads strictly SHALL keep the fields and the digest
which that release reads. A setting that AIGW newly owns in that file outside what that digest covers SHALL have a
record of its own beside the first, whose reader keeps an entry that it does not know and leaves that entry's
setting as it is. Release acceptance SHALL roll the program back to the published predecessor with every newly
owned setting written.

#### Scenario: The previous release synchronizes after a rollback

- **GIVEN** the newer program wrote Claude Code's settings with a map
- **WHEN** the program is rolled back to the previous release and that release synchronizes
- **THEN** the synchronization SHALL succeed with Claude Code enabled
- **AND** the map SHALL stay in the settings as the newer program wrote it.

#### Scenario: A new setting lies inside what the record's digest covers

- **GIVEN** the newer program wrote each model's context window inside the provider entries of Hermes's file, which
  the digest of Hermes's record covers, and into Codex's model table, whose record holds the table's digest
- **WHEN** the program is rolled back to the previous release and that release synchronizes
- **THEN** the synchronization SHALL succeed with Hermes and Codex enabled, with no second record
- **AND** the previous release SHALL find both files its own and write its own form of them.

#### Scenario: The newer program returns after a rollback

- **WHEN** the newer program is activated again after the previous release synchronized, and synchronizes
- **THEN** it SHALL find the map its own, however the previous release re-indented the settings, and write the
  selection's map.

#### Scenario: The previous release withdrew the client

- **GIVEN** the previous release withdrew Claude Code after a rollback, which left the map and its record
- **WHEN** the newer program next projects or withdraws Claude Code, or is uninstalled
- **THEN** the map SHALL return to what the settings held before AIGW wrote it, and its record SHALL be removed.

#### Scenario: The record holds an entry of a later release

- **WHEN** the record of newly owned settings holds an entry that this program does not know
- **THEN** this program SHALL keep the entry when it writes the record and SHALL leave that entry's setting as it
  is.
