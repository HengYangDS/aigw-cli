## ADDED Requirements

### Requirement: A Hermes configuration keeps what AIGW does not own

When AIGW writes Hermes's configuration file, it SHALL change only the model selection and the providers that
it owns. The other keys SHALL keep their values, their order and their quoting, and AIGW SHALL NOT wrap a value
onto a second line. AIGW's ownership record SHALL have one form that a change of the YAML library does not alter.

#### Scenario: The operator's file holds quoted strings, block scalars and long values

- **WHEN** AIGW projects its providers into a Hermes configuration whose other keys carry quoted strings,
  block scalars and values longer than 80 characters
- **THEN** those keys keep their values, their order and their quoting
- **AND** no value is wrapped onto a second line.

#### Scenario: A block scalar keeps its value

- **WHEN** AIGW projects its providers into a Hermes configuration that holds a folded block scalar which keeps
  its final line breaks, a folded block scalar with a more-indented line, or a literal block scalar with trailing
  spaces
- **THEN** the scalar's value is unchanged after the projection and after its withdrawal.

#### Scenario: The ownership record is written by a later build

- **WHEN** a build of AIGW with another release of its YAML library writes the record of the model selection
  that it replaced
- **THEN** the record has the bytes that the earlier build wrote for the same file
- **AND** the earlier build reads it.
