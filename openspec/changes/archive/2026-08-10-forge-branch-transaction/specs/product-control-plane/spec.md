## ADDED Requirements

### Requirement: Local release-root promotion

AIGW SHALL advance local release `main` from accepted `dev` only through one
explicit exact compare-and-swap transaction. The transaction SHALL verify a
clean repository, exact observed `main` and `dev` commits, and ancestry before
moving `main`. It SHALL NOT push a remote, create a tag, publish a release, or
change `dev`.

#### Scenario: Accepted dev is ready for release

- **WHEN** `main` and `dev` match the operator's exact observations
- **AND** `main` is an ancestor of `dev`
- **THEN** one compare-and-swap SHALL advance `main` to exactly `dev`
- **AND** `dev` SHALL remain unchanged.

#### Scenario: Release coordinates drift

- **WHEN** observed `main` or `dev` differs from the supplied coordinate
- **THEN** promotion SHALL fail before any ref update.

#### Scenario: Release history diverges

- **WHEN** local `main` is not an ancestor of accepted `dev`
- **THEN** promotion SHALL fail before any ref update.

#### Scenario: Release main already equals accepted dev

- **WHEN** exact `main` and `dev` identify the same commit
- **THEN** promotion SHALL report an idempotent success without changing refs.
