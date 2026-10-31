## ADDED Requirements

### Requirement: Program commands preview their effect

`install`, `update` and `uninstall` with `--dry-run` SHALL report the program
file, the version and the rollback copy that they would write or remove.
`update --dry-run` SHALL read only public release metadata, and SHALL NOT
download, verify or run an archive or change a Keychain item.

#### Scenario: An update is previewed

- **WHEN** `aigw update --dry-run` runs while a newer release is available
- **THEN** it SHALL report the installed and the newer version
- **AND** no file, rollback copy or Keychain item SHALL change.

#### Scenario: An uninstall is previewed

- **WHEN** `aigw uninstall --dry-run` runs
- **THEN** it SHALL report the program file and the rollback copy that it would
  remove
- **AND** both SHALL remain.
