# projection-format Specification

## Purpose

Define the canonical text layout of repository-projected specifications so
generated and reviewed source remain byte-stable.

## Requirements

### Requirement: Projected specifications follow canonical text layout

A projected OpenSpec specification SHALL end with exactly one newline and no
terminal blank line.

#### Scenario: Archive projection is verified

- **WHEN** an archived Change updates a canonical specification
- **THEN** the projected specification satisfies the repository text-layout rule
