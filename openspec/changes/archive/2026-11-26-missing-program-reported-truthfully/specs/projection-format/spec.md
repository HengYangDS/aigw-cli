## ADDED Requirements

### Requirement: Codex's model table names the program that produced it

AIGW's record of a Codex model table SHALL name the program that produced it: its path as configured, the SHA-256 of
its file and the version that its package manifest declares, when one is readable. `aigw check --depth local` SHALL
compare that program with the configured one by path, digest and manifest version, running no program while they
agree, and SHALL fail with repair `aigw sync` on any difference that a sync can repair.

#### Scenario: Sync records the producer

- **WHEN** `aigw sync` writes Codex's model table
- **THEN** AIGW's record SHALL name the configured path, the SHA-256 and the package manifest's version of the program
  that produced the table.

#### Scenario: The program changes in place

- **GIVEN** a model table that the configured program produced
- **WHEN** the file at the configured path is replaced by other bytes
- **THEN** `aigw check --depth local` SHALL fail, saying that the table came from another program, with repair
  `aigw sync`
- **AND** the check SHALL run no program but the read-only probes that tell whether a sync can produce a table.

#### Scenario: Another program is configured

- **WHEN** the configured path is not the one that AIGW's record names
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`.

#### Scenario: A launcher keeps its bytes while its package changes

- **GIVEN** the configured program's package manifest declared the version that AIGW's record holds
- **WHEN** the manifest declares another version while the program's file keeps its bytes
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`.

#### Scenario: A program without a manifest

- **WHEN** no package manifest of the configured program is readable
- **THEN** `aigw check --depth local` SHALL compare the program by path and digest only.

#### Scenario: A record names no producer

- **WHEN** AIGW's record of a Codex model table names no producing program, as a record of an earlier release does
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`
- **AND** the next `aigw sync` SHALL write the producer into the record.
