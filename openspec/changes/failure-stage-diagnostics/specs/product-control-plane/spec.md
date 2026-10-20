## ADDED Requirements

### Requirement: Local failures name the failed stage

When an AIGW command fails at a local file operation or while loading its
configuration, its human and JSON diagnostic SHALL name the failed operation
and the operating-system error, or the configuration stage that failed. A parse
failure SHALL name the line and column, and a validation failure SHALL name the
running AIGW version. The diagnostic SHALL NOT contain a path, configuration
content, a credential or an internal error message.

#### Scenario: A local file operation fails

- **WHEN** a command fails because a file operation returns an
  operating-system error
- **THEN** the diagnostic SHALL name the operation and the error
- **AND** it SHALL NOT contain the file path.

#### Scenario: The configuration cannot be parsed

- **WHEN** the configuration is not valid TOML
- **THEN** the diagnostic SHALL name the parse stage with its line and column
- **AND** it SHALL NOT contain configuration content.

#### Scenario: The configuration fails validation

- **WHEN** the configuration does not satisfy the running AIGW's schema
- **THEN** the diagnostic SHALL name the validate stage and the running AIGW
  version
- **AND** its next step SHALL name `aigw doctor` and, for a configuration that
  a newer AIGW wrote, the installed AIGW.
