## ADDED Requirements

### Requirement: Diagnostic assertions read text, not terminal presentation

Tests and gates that assert a native tool's diagnostics SHALL judge the
diagnostic text after removing terminal escape sequences. A hosted runner that
colors the output SHALL reach the same judgement as an uncolored local
terminal.

#### Scenario: A hosted runner colors a native tool's warning

- **WHEN** a native tool prints an expected warning with ANSI escape sequences
  between its words
- **THEN** the assertion SHALL count that warning exactly once
- **AND** the same output without escape sequences SHALL yield the same count.
