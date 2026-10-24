## ADDED Requirements

### Requirement: Hermes verification releases its gateway before deleting its home

Hermes verification SHALL bound the idle lifetime of the gateway its chat
starts, keep that gateway's host locks inside the verification home, and
delete the home only after observing that no process holds its gateway lock.
When the lock stays held after one native stop request, verification SHALL
retain the home, name it, and report incomplete verification.

#### Scenario: The verification gateway exits after the chat

- **WHEN** the verification chat returns and its gateway exits on idle
- **THEN** verification SHALL delete its home only after the gateway lock is
  free
- **AND** the operator's host lock directory SHALL remain untouched.

#### Scenario: The verification gateway outlives its idle bound

- **WHEN** the gateway lock is still held after the idle bound
- **THEN** verification SHALL request `hermes gateway stop` once with closed
  input for that home and wait again
- **AND** it SHALL send no signal to the gateway.

#### Scenario: The verification gateway keeps running

- **WHEN** the gateway lock is still held after the stop request
- **THEN** verification SHALL retain the home and name it in its error
- **AND** it SHALL NOT report completed verification.
