## ADDED Requirements

### Requirement: A client's file is one file however its path is spelled

Every decision of a command about which of a client's configuration files it keeps, drops or adds SHALL compare files
by one judgement: two configured paths that both exist SHALL name one file when the operating system reports one
file, and otherwise when their cleaned spellings are equal. A file kept in another spelling SHALL NOT be treated as
dropped, with or without the provider's Token.

#### Scenario: A rollback meets the file in another spelling

- **WHEN** the configuration that a rollback restores names a client's file by a symbolic link and the current one
  names it by its resolved path, or the reverse, and the provider's Token is not stored
- **THEN** the rollback SHALL keep that file as the client's file and SHALL NOT remove AIGW's settings or record from it
- **AND** a model that the person chose in that file SHALL be treated as it is when both name the file alike.

#### Scenario: An edit names the file in another spelling

- **WHEN** `aigw client edit <client> --target` names the file that the client already has, in another spelling
- **THEN** the edit SHALL end the same way with the provider's Token and without it.

#### Scenario: A file that does not exist yet

- **WHEN** a configured path names no existing file
- **THEN** it SHALL name the same file as another path only when their cleaned spellings are equal.
