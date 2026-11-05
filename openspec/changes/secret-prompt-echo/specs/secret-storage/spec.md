## ADDED Requirements

### Requirement: Interactive Token input is never echoed

A Token prompt at an interactive terminal SHALL read the Token with terminal
echo disabled in every prompt mode, including the accessible mode that
`NO_COLOR` or `AIGW_ACCESSIBLE` selects, and SHALL NOT write the Token to the
prompt output. Token input that is not an interactive terminal SHALL be read
without being written to the prompt output.

#### Scenario: An accessible prompt reads a Token at a terminal

- **WHEN** `NO_COLOR` or `AIGW_ACCESSIBLE` selects accessible prompts and the
  operator types a Token at an interactive terminal
- **THEN** the terminal does not echo the Token
- **AND** the prompt output contains only the prompt label and a line ending.

#### Scenario: An accessible prompt reads a Token from a pipe

- **WHEN** accessible prompts read a Token from standard input that is not a
  terminal
- **THEN** the Token is read as one line
- **AND** the prompt output does not contain it.
