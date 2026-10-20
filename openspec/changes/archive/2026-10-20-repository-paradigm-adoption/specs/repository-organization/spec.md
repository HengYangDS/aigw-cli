## ADDED Requirements

### Requirement: Decision records share one rationale structure

Each decision record SHALL declare its record identity and an accepted
disposition in its front matter and SHALL hold exactly the sections Context,
Decision, Alternatives Rejected, Consequences and Boundary, and Evidence and
Revisit, each with authored rationale and no lower headings. A record SHALL
NOT carry execution content such as code blocks, task lists, inline HTML or
command invocations.

#### Scenario: A decision record is validated

- **WHEN** the repository's governance reads a record under `docs/decisions`
- **THEN** its front matter SHALL name the identity that matches its file
  sequence and the accepted disposition
- **AND** its body SHALL hold the five rationale sections in order.

#### Scenario: A record needs to name a command

- **WHEN** a decision depends on a product command
- **THEN** the record SHALL describe the command's behavior in prose and leave
  its usage to the guides instead of carrying the invocation.

### Requirement: Repository declarations follow the shared paradigm

The repository SHALL NOT restate a role or policy that the shared ETHOS
paradigm already owns. Its release mirror SHALL be declared in the release
declaration. Commit-provenance verification SHALL require the shared SSH
signature policy when the repository declares no commit policy, and a declared
commit policy MAY only restrict commit scopes.

#### Scenario: A workspace table restates signing

- **WHEN** a tracked commit policy declares its own signing requirement or
  format
- **THEN** commit-provenance verification SHALL refuse that policy.

#### Scenario: No commit policy is declared

- **WHEN** the verified commit carries no workspace table
- **THEN** verification SHALL apply the shared subject grammar and SSH
  signature requirement to every commit in the range.
