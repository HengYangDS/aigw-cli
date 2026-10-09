## MODIFIED Requirements

### Requirement: Projection conflicts are rejected before configuration writes

The synchronization transaction SHALL preflight every participating client before
persisting configuration or recovery changes. A Codex forwarding destination
SHALL participate in the same guarded snapshot, backup, checkpoint, and
compensation boundary without changing the predecessor-readable main schema.
Failures SHALL compensate only captured owned postimages, never rerun inverse
reconciliation against rejected state.

#### Scenario: Preflight finds a conflicting client projection

- **WHEN** a client rejects the planned transition
- **THEN** configuration and client files remain unchanged
- **AND** the result reports preflight rejection, not a rollback attempt.

#### Scenario: An applied transition is compensated

- **WHEN** a later write fails after a projection was applied
- **THEN** compensation restores the exact observed preimage, including user edits
- **AND** a concurrent edit to the postimage is preserved and reported as a conflict.

#### Scenario: One owned file conflicts during compensation

- **WHEN** an external writer changes one file after AIGW applied a transition
- **THEN** compensation preserves that file and restores every independent file
  still matching AIGW's postimage
- **AND** configuration, backup, checkpoint, and client sidecar restoration SHALL
  continue after a conflict and identify every encountered cause.

#### Scenario: A retained credential reader observes forwarding

- **WHEN** the installed successor selects a Codex forwarding destination
- **THEN** the original 0.3.1 reader SHALL still decode the main configuration
  and resolve the same Account credential identity
- **AND** the destination SHALL remain bound to its recorded upstream identity.

#### Scenario: Restore direct mode before predecessor rollback

- **WHEN** the successor restores direct routing and completes native verification
- **THEN** the predecessor SHALL read configuration, verified checkpoint, and
  rollback state through its original public commands
- **AND** active forwarding state SHALL be absent
- **AND** required recovery material SHALL remain until its owner releases it.

#### Scenario: Forwarding changes during verification or commit

- **WHEN** forwarding state no longer matches the captured preimage
- **THEN** AIGW SHALL reject the stale commit or checkpoint
- **AND** SHALL preserve the newer state and independently restore unchanged
  files owned by its failed attempt.
