## MODIFIED Requirements

### Requirement: Diagnostic assertions read text, not terminal presentation

Tests and gates that assert a native tool's diagnostics SHALL judge the
diagnostic text after removing terminal escape sequences. A hosted runner that
colors the output SHALL reach the same judgement as an uncolored local
terminal. Tests that assert AIGW's rendered output SHALL reach the same
judgement wherever the renderer breaks a line.

#### Scenario: A hosted runner colors a native tool's warning

- **WHEN** a native tool prints an expected warning with ANSI escape sequences
  between its words
- **THEN** the assertion SHALL count that warning exactly once
- **AND** the same output without escape sequences SHALL yield the same count.

#### Scenario: A rendered path wraps at the terminal width

- **WHEN** a test renders a path wider than the terminal width, so that the
  position of its line break depends on the length of the host's temporary
  directory
- **THEN** the assertion SHALL find the complete path at every break position.
