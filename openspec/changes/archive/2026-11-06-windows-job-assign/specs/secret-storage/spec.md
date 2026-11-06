## MODIFIED Requirements

### Requirement: Native credential operations are bounded

AIGW SHALL run each native credential operation in a private subprocess with a
five-second deadline and bounded cleanup, passing only the platform identity its
store requires. A timeout or authorization failure SHALL end the exact operation
without retry, fallback, access-policy change, or backend migration, apart from
the macOS retained-item handling below. Creation or timing SHALL NOT prove later
authorization.

#### Scenario: The installed executable creates and rotates a credential

- **WHEN** its native writer creates or updates an authorized exact slot
- **THEN** its native reader returns the written value without interaction
- **AND** mutation emits no Token, including unexpected backend output.

#### Scenario: Exact deletion is repeated

- **WHEN** an authorized deletion removes an item and deletion is requested again
- **THEN** both operations succeed and the exact item remains absent.

#### Scenario: Credential presence is observed

- **WHEN** AIGW checks an exact native credential slot
- **THEN** the subprocess returns only present or absent metadata
- **AND** no credential value enters the parent process.

#### Scenario: A credential operation fails or requires authorization

- **WHEN** the exact native item cannot be accessed within the admitted operation
  and is neither a retained item that the installed program migrates nor one
  that another AIGW program reads through `/usr/bin/security`
- **THEN** the subprocess fails without retrying or returning a Token
- **AND** no credential or access-control state changes
- **AND** any operating-system authorization UI remains an explicit platform
  behavior rather than a suppressed or disproved event.

#### Scenario: A concurrent migration completes between lookups

- **WHEN** a macOS worker is denied or misses the exact slot and then finds
  neither a retained nor a staged item for it
- **THEN** it SHALL read the slot once more without UI, because another worker
  may have just recreated it as an AIGW-owned item
- **AND** SHALL return that read's result without a further attempt.

#### Scenario: The native service does not answer

- **WHEN** the operation reaches its deadline
- **THEN** the parent terminates and reaps its subprocess through bounded cleanup
- **AND** the helper reports the deadline rather than suggesting configuration sync.

#### Scenario: A credential subprocess exits at once

- **WHEN** a native credential subprocess, or a client credential command that
  AIGW runs, exits as soon as it starts or starts its own children at once
- **THEN** it SHALL already belong to the process group or Job that owns its
  cleanup before it runs any code, and its start SHALL succeed
- **AND** a subprocess that cannot be contained SHALL be stopped, with the
  cause reported.
