## ADDED Requirements

### Requirement: The shipped team file declares a window only below the default

The shipped team file SHALL declare `context_window` for a model only where the model's vendor documents a context
window below 1,000,000, or where no vendor document remains and another named source gives a lower number. It SHALL
declare that number, the same for every model of one family. The file SHALL stay the byte-identical native export,
which holds no comment, so the repository's dated qualification evidence SHALL name each such family's source.

#### Scenario: Declared windows are below the default and agree within a family

- **WHEN** the shipped team file is read
- **THEN** every `context_window` in it SHALL be below 1,000,000
- **AND** the models of one family SHALL declare the same window or none.
