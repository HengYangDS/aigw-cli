## ADDED Requirements

### Requirement: A Hermes configuration keeps what AIGW does not own

When AIGW writes Hermes's configuration file, it SHALL change only the model selection and the providers that
it owns. The other keys SHALL keep their values, their order, their comments and their quoting, and AIGW SHALL
NOT wrap a value onto a second line. AIGW's ownership record SHALL have one form that a change of the YAML
library does not alter.

#### Scenario: The operator's file holds comments and long values

- **WHEN** AIGW projects its providers into a Hermes configuration whose other keys carry comments, quoted
  strings, block scalars and values longer than 80 characters
- **THEN** those keys keep their values, their order, their comments and their quoting
- **AND** no value is wrapped onto a second line.

#### Scenario: The ownership record is written by a later build

- **WHEN** a build of AIGW with another release of its YAML library writes the record of the model selection
  that it replaced
- **THEN** the record has the bytes that the earlier build wrote for the same file
- **AND** the earlier build reads it.
