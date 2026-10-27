## MODIFIED Requirements

### Requirement: Projection conflicts are rejected before configuration writes

The synchronization transaction SHALL preflight every participating client before
persisting configuration, backup, or checkpoint changes. A forwarding selection
SHALL share the same snapshot, backup, checkpoint and compensation boundary
without changing the predecessor-readable main schema. Failures after actual
writes SHALL compensate only captured owned postimages, never re-run an inverse
reconciliation against the same rejected state.

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
  that still matches AIGW's postimage
- **AND** configuration, backup, checkpoint and client sidecar cleanup do not stop
  at the first conflict
- **AND** all encountered causes remain identifiable in the returned error.

#### Scenario: A retained credential reader observes forwarding

- **WHEN** the installed successor selects a forwarding destination for a client
- **THEN** a retained predecessor reader SHALL still decode the main configuration
  and the verified checkpoint, and resolve the same Account credential identity
- **AND** the destination SHALL remain bound to its recorded upstream identity.

#### Scenario: Forwarding changes during verification or commit

- **WHEN** forwarding state no longer matches the captured preimage
- **THEN** AIGW SHALL reject the stale commit or checkpoint
- **AND** SHALL preserve the newer state and restore only the unchanged files
  that its failed attempt owns.
