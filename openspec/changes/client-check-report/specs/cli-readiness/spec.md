## MODIFIED Requirements

### Requirement: Native warning symbols are diagnostics

A native diagnostic line whose first visible character is an explicit warning
or error symbol, `⚠`, `✗`, `✖`, `✘` or `❌`, SHALL count as a warning in
every check that rejects explicit native warnings. Check marks, progress
glyphs and symbols inside a line SHALL NOT.

#### Scenario: Hermes reports an out-of-sync installation

- **WHEN** Hermes exits zero after printing a line that starts with `⚠`
- **THEN** verification SHALL report incomplete qualification in its own
  words, and the client's line SHALL appear only on request.

#### Scenario: A client prints ordinary progress

- **WHEN** a client prints `session_id:`, a `✓` result or a `↻` resume line
- **THEN** that line SHALL NOT be a warning.
