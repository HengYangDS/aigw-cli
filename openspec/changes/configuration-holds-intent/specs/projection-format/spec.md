## MODIFIED Requirements

### Requirement: Credential commands match a provider endpoint

Codex SHALL receive one provider table, AIGW's own `aigw` provider, which SHALL authenticate through the absolute AIGW
command, whose fingerprint names its client, provider and endpoint. The command SHALL return the Token when that
fingerprint matches an enabled client, a configured provider and an endpoint of one of that provider's models, and
SHALL check no selection; `credential-continuity`'s frozen view owns this. AIGW SHALL neither invoke native login nor
change client credentials.

#### Scenario: A Codex provider uses a provider Token

- **WHEN** AIGW synchronizes Codex's provider table
- **THEN** the table SHALL be named `aigw` and declare `wire_api = "responses"`
- **AND** its auth command invokes the absolute AIGW executable with
  `credential codex <fingerprint>`
- **AND** no Token value is copied into client files.

#### Scenario: Return to the default provider

- **GIVEN** AIGW's record of a Codex configuration file names a provider table other than `aigw`, which an earlier
  release wrote for an explicit `model_provider`
- **WHEN** AIGW next synchronizes Codex
- **THEN** that table SHALL be removed in the same transaction that writes the `aigw` table
- **AND** Codex SHALL authenticate through the `aigw` table's credential command.

#### Scenario: A running session outlives its client's move

- **GIVEN** a running session holds the credential command of one provider
- **WHEN** its client selects a model of another provider, and the first
  provider still has a model on that endpoint
- **THEN** the command SHALL keep returning the first provider's Token.

#### Scenario: A retained command matches no provider endpoint

- **WHEN** a credential command's fingerprint matches no configured provider and
  endpoint of one of that provider's models
- **THEN** it returns a synchronization and reload instruction
- **AND** reads no Token and writes no credential bytes to standard output.

#### Scenario: Model or label changes without a credential-identity change

- **WHEN** only the selected model of the same provider and endpoint, or its
  label, changes
- **THEN** the existing fingerprint still matches
- **AND** the command reads the same provider's current Token
- **AND** naming and catalog selection SHALL NOT alter credential ownership.

#### Scenario: Multiple Codex homes use the same provider Token

- **WHEN** setup discovers multiple supported, AIGW-owned Codex targets
- **THEN** it writes the matching credential command into each target
- **AND** reads or stores only the selected provider's Token
- **AND** leaves client-owned credentials unchanged.

### Requirement: Codex's model table carries the selected model's declared context window

In the model table that AIGW projects for Codex, the entry that describes the selected model, its alias or the
model's own entry, SHALL carry the model's declared context window as its `context_window` and as its
`max_context_window`. Every other field of that entry and every other entry SHALL stay as the client's own table has
them. For each Codex target, `aigw check codex` SHALL report the window that Codex is given, or why AIGW gives it none.

#### Scenario: An alias carries the declared window

- **GIVEN** the client's bundled table holds the selected model's family but not its ID
- **WHEN** AIGW projects Codex's files
- **THEN** the alias that AIGW adds for the model SHALL carry the declared window in both fields
- **AND** the family's own entry and every other entry SHALL be unchanged.

#### Scenario: The selected model is an entry of the client's table

- **WHEN** the selected model's ID is itself an entry of the client's bundled table
- **THEN** AIGW SHALL project the complete bundled table with that entry's two window fields set to the model's
  declared context window
- **AND** nothing else of the table SHALL change.

#### Scenario: A check reports the window that Codex is given

- **GIVEN** the table that AIGW wrote for a Codex target holds an entry for the selected model
- **WHEN** `aigw check codex` runs
- **THEN** a check of its own SHALL pass and report that entry's context window
- **AND** it SHALL name the target as the other local checks of that target do.

#### Scenario: A check finds a window other than the declared one

- **GIVEN** the table that AIGW wrote for a Codex target holds an entry for the selected model whose
  `context_window` or `max_context_window` is not the model's declared context window
- **WHEN** `aigw check codex` runs
- **THEN** the check of its own SHALL fail and report both fields as the table carries them beside the declared window
- **AND** its repair SHALL be `aigw sync`, which writes the table again.

#### Scenario: The client's table does not hold the model's family

- **WHEN** the selected model's family is not in the client's bundled table
- **THEN** AIGW SHALL project the table without an entry for the model
- **AND** `aigw check codex` SHALL report, in a check of its own that passes, that Codex's table has no entry for the
  model's family, so Codex chooses the window itself whatever the model declares.

#### Scenario: AIGW writes no table for a Codex target

- **WHEN** a Codex target's configuration names a `model_catalog_json` that AIGW did not write
- **THEN** AIGW SHALL write no table for that target
- **AND** `aigw check codex` SHALL report, in a check of its own that passes, that Codex is not given the declared
  window, with that reason.

#### Scenario: The declared window of the selected model changes

- **WHEN** `aigw model edit` changes the declared window of the model that Codex selects
- **THEN** the command SHALL rewrite Codex's table with the new window
- **AND** `aigw sync --dry-run` afterwards SHALL plan no change.

### Requirement: Codex's model table names the program that produced it

AIGW's record of a Codex model table SHALL name the program that produced it: its path as resolved, the SHA-256 of its
file and the version that its package manifest declares, when one is readable. `aigw check --depth local` SHALL
compare that program with the one that Codex's declaration resolves to now by path, digest and manifest version,
running no program while they agree, and SHALL fail with repair `aigw sync` on any difference that a sync can repair.

#### Scenario: Sync records the producer

- **WHEN** `aigw sync` writes Codex's model table
- **THEN** AIGW's record SHALL name the resolved path, the SHA-256 and the package manifest's version of the program
  that produced the table.

#### Scenario: The program changes in place

- **GIVEN** a model table that Codex's program produced
- **WHEN** the file at the program's path is replaced by other bytes
- **THEN** `aigw check --depth local` SHALL fail, saying that the table came from another program, with repair
  `aigw sync`
- **AND** the check SHALL run no program but the read-only probes that tell whether a sync can produce a table.

#### Scenario: Another program is configured

- **WHEN** the path that Codex's declaration resolves to is not the one that AIGW's record names
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`.

#### Scenario: A launcher keeps its bytes while its package changes

- **GIVEN** the package manifest of Codex's program declared the version that AIGW's record holds
- **WHEN** the manifest declares another version while the program's file keeps its bytes
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`.

#### Scenario: A program without a manifest

- **WHEN** no package manifest of Codex's program is readable
- **THEN** `aigw check --depth local` SHALL compare the program by path and digest only.

#### Scenario: A record names no producer

- **WHEN** AIGW's record of a Codex model table names no producing program, as a record of an earlier release does
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`
- **AND** the next `aigw sync` SHALL write the producer into the record.

#### Scenario: The editor installs a newer version of Codex's extension

- **GIVEN** Codex declares `extensions`, and a sync wrote its model table with the program of the highest version then
  installed
- **WHEN** the editor installs a higher version
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`
- **AND** the next `aigw sync` SHALL write the table with the new version's program and name it in the record.
