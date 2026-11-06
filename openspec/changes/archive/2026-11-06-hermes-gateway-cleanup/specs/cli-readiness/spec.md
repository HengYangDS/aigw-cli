## MODIFIED Requirements

### Requirement: Hermes verification waits for its gateway process to exit

Hermes verification SHALL delete its disposable home only after every gateway
that Hermes's own records place in it has ended: the free gateway lock, the
process that `gateway_state.json` records, and a launched gateway's lifecycle
record. A gateway that outlives the bound SHALL leave the home in place and
named in the error.

#### Scenario: The gateway releases its lock before it exits

- **WHEN** the gateway lock is free but the recorded gateway process still runs
- **THEN** verification SHALL wait for that process to exit before it deletes
  the home
- **AND** verification SHALL keep the home and name it in the error when the
  process outlives the bound.

#### Scenario: No gateway recorded a run

- **WHEN** the home has no gateway state file and no gateway launch log
- **THEN** the free gateway lock alone SHALL decide whether the home is deleted.

#### Scenario: A launched gateway is still starting

- **WHEN** verification ends while a gateway launched into its home has written
  its launch log but has not yet locked or recorded itself
- **THEN** verification SHALL keep the home until the gateway's lifecycle record
  shows it exited, or names a process that is gone
- **AND** if the bound passes first, verification SHALL retain the home and name
  it in an error that does not claim the gateway still runs.
