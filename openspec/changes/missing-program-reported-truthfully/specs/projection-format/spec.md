## ADDED Requirements

### Requirement: Codex's model table names the program that produced it

AIGW's record of a Codex model table SHALL name the program that produced it: its path as configured, the version
that it printed and the SHA-256 of its file. `aigw check --depth local` SHALL compare that program with the configured
one by path, by digest and by the version that Codex's package manifest declares, when one is readable, without
running a program, and SHALL fail with repair `aigw sync` on any difference.

#### Scenario: Sync records the producer

- **WHEN** `aigw sync` writes Codex's model table
- **THEN** AIGW's record SHALL name the configured path, the version and the SHA-256 of the program that produced the
  table.

#### Scenario: The program changes in place

- **GIVEN** a model table that the configured program produced
- **WHEN** the file at the configured path is replaced by other bytes
- **THEN** `aigw check --depth local` SHALL fail, saying that the table came from another program, with repair
  `aigw sync`
- **AND** the check SHALL run no program.

#### Scenario: Another program is configured

- **WHEN** the configured path is not the one that AIGW's record names
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`.

#### Scenario: A launcher keeps its bytes while its package changes

- **GIVEN** the package manifest beside the configured program declared the version that AIGW's record holds
- **WHEN** the manifest declares another version while the program's file keeps its bytes
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`.

#### Scenario: A program without a manifest

- **WHEN** no package manifest of the configured program is readable
- **THEN** `aigw check --depth local` SHALL compare the program by path and digest only.

#### Scenario: A record names no producer

- **WHEN** AIGW's record of a Codex model table names no producing program, as a record of an earlier release does
- **THEN** `aigw check --depth local` SHALL fail with repair `aigw sync`
- **AND** the next `aigw sync` SHALL write the producer into the record.
