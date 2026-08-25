## MODIFIED Requirements

### Requirement: accepted ref parity is visible without duplicate proof

When a maintainer publication atomically advances peer `main` and `dev` to one
accepted product object, each peer SHALL expose one `main` result that proves
both protected refs resolve to the event's exact commit. The same `main` event
SHALL own the complete verification graph. A proposal merge into `dev` SHALL
remain a reviewed development state and SHALL NOT be interpreted as accepted
publication merely because it updates the protected review branch.

#### Scenario: a developer proposal is merged into dev

- **WHEN** a reviewed proposal advances peer `dev`
- **THEN** the completed pull-request or merge-request graph SHALL remain its
  verification evidence
- **AND** the resulting `dev` push SHALL NOT require `main` parity
- **AND** the resulting `dev` push SHALL NOT repeat the complete graph.

#### Scenario: a peer receives one accepted object on main and dev

- **WHEN** an atomic maintainer publication advances peer `main` and `dev`
- **THEN** the `main` event SHALL run the complete verification graph
- **AND** the same `main` event SHALL require both protected refs to equal its
  exact commit
- **AND** no second graph or parity-only `dev` pipeline SHALL be required.
