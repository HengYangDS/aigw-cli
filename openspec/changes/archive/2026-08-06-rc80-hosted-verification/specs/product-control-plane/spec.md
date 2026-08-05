## ADDED Requirements

### Requirement: Hosted evidence identity is Forge-portable
A hosted governance job SHALL validate tracked evidence by its recorded content
tree within the current Forge's own history.

#### Scenario: Evidence records a commit from the peer Forge
- **WHEN** tracked evidence names a commit object absent from the current Forge
- **THEN** the job SHALL accept the evidence only if the recorded tree exists in
  the current `HEAD` ancestry
- **AND** it SHALL NOT fetch the peer Forge or require a cross-Forge commit map.

#### Scenario: The recorded commit object is locally available
- **WHEN** the recorded commit resolves in the current repository
- **THEN** its tree SHALL equal the recorded tree
- **AND** the recorded tree SHALL still exist in the current `HEAD` ancestry.

### Requirement: Native client fixtures are repository-controlled
Cross-platform tests SHALL construct client executables from test-owned
fixtures rather than borrowing unrelated host toolchain executables.

#### Scenario: Windows tests an unreadable Claude launcher
- **WHEN** native Windows verification exercises launcher read failures
- **THEN** the fixture SHALL be an isolated executable controlled by the test
- **AND** the result SHALL not depend on the installed Go toolchain path or contents.
