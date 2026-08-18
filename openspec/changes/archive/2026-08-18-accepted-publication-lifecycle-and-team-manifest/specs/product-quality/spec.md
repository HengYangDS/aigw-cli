## ADDED Requirements

### Requirement: Accepted publication trees contain only archived Changes

The source gate SHALL admit an accepted publication tree only when
`openspec/changes/` contains no active Change directories. Completed Change
artifacts SHALL be archived before `dev`, `main`, or a release tag is accepted.

#### Scenario: Active Change reaches source verification

- **WHEN** source verification observes an active Change directory
- **THEN** verification SHALL fail with the active Change names
- **AND** SHALL direct the maintainer to archive completed Changes before publication
