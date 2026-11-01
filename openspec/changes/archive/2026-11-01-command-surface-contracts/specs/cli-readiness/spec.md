## ADDED Requirements

### Requirement: Readiness reports an unreadable configuration's stage

`status`, `check` and `doctor` SHALL report a configuration that cannot be
loaded with the stage that failed, read, parse or validate, and a parse
failure's line and column, in human and JSON output, and SHALL exit 1.

#### Scenario: The configuration cannot be parsed

- **WHEN** `status`, `check` or `doctor` runs with `--json` and the
  configuration is not valid TOML
- **THEN** each SHALL report `configuration_parse` with the line and column
- **AND** each SHALL exit 1.
