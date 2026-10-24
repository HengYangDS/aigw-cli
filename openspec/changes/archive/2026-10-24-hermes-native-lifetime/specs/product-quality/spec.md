## ADDED Requirements

### Requirement: Native warning symbols are diagnostics

A native diagnostic line whose first visible character is an explicit warning
or error symbol, `⚠`, `✗`, `✖`, `✘` or `❌`, SHALL count as a warning in
every check that rejects explicit native warnings. Check marks, progress
glyphs and symbols inside a line SHALL NOT.

#### Scenario: Hermes reports an out-of-sync installation

- **WHEN** Hermes exits zero after printing a line that starts with `⚠`
- **THEN** verification SHALL report incomplete qualification without the
  private diagnostic.

#### Scenario: A client prints ordinary progress

- **WHEN** a client prints `session_id:`, a `✓` result or a `↻` resume line
- **THEN** that line SHALL NOT be a warning.

### Requirement: The Hermes release journey resumes its session natively

The native release journey SHALL create its Hermes session once with its
creation options, resume it by the stored session identifier without options
that Hermes freezes at creation, and stop the journey's gateway before
deleting the journey home.

#### Scenario: Continue the Hermes session across program lifecycle steps

- **WHEN** the journey continues its Hermes session after an update, rollback
  or re-upgrade
- **THEN** it SHALL pass `--resume` with the stored session identifier
- **AND** Hermes SHALL retain the earlier tool result in the continued input.
