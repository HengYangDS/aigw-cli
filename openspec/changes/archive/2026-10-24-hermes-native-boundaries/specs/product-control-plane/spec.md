## ADDED Requirements

### Requirement: Hermes verification waits for its gateway process to exit

Hermes verification SHALL delete its disposable home only after the gateway
process that the home's `gateway_state.json` records has exited, in addition to
the free gateway lock. Hermes releases that lock before it writes its exit
records and logs into the home, and keeps the state file after the gateway
stops. A process that outlives the bound SHALL leave the home in place and
named in the error.

#### Scenario: The gateway releases its lock before it exits

- **WHEN** the gateway lock is free but the recorded gateway process still runs
- **THEN** verification SHALL wait for that process to exit before it deletes
  the home
- **AND** verification SHALL keep the home and name it in the error when the
  process outlives the bound.

#### Scenario: No gateway recorded a run

- **WHEN** the home has no gateway state file
- **THEN** the free gateway lock alone SHALL decide whether the home is deleted.
