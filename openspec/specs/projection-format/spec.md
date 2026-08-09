# projection-format Specification

## Purpose
TBD - created by archiving change projection-format-hygiene. Update Purpose after archive.
## Requirements
### Requirement: Projected specifications follow canonical text layout

A projected OpenSpec specification SHALL end with exactly one newline and no
terminal blank line.

#### Scenario: Archive projection is verified

- **WHEN** an archived Change updates a canonical specification
- **THEN** the projected specification satisfies the repository text-layout rule
