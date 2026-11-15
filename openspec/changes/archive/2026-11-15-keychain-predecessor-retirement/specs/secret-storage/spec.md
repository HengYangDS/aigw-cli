## ADDED Requirements

### Requirement: Every native credential operation is bounded

AIGW SHALL run each native credential operation in a private subprocess with a
five-second deadline and bounded cleanup, passing only the platform identity its
store requires. A timeout or authorization failure SHALL end the exact operation
without retry, fallback, access-policy change, or backend migration. Creation
or timing SHALL NOT prove later authorization.

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

- **WHEN** the platform refuses the exact native item, or would need a person
  to admit the operation
- **THEN** the subprocess fails without running the operation again and
  without returning a Token
- **AND** no credential or access-control state changes
- **AND** any operating-system authorization UI remains an explicit platform
  behavior rather than a suppressed or disproved event.

#### Scenario: A new macOS item is created

- **WHEN** the macOS writer creates a slot's item
- **THEN** it SHALL store the value and the purpose label in one Keychain call
- **AND** the item's access description SHALL be that label.

#### Scenario: Several writers write one slot at once

- **WHEN** two native writers create the same absent slot, or three replace
  the same slot, at the same time
- **THEN** each SHALL succeed and the slot SHALL hold one of their values
- **AND** one item SHALL exist for the slot, with its purpose label.

#### Scenario: Another writer changes the slot during a mutation

- **WHEN** another process replaces, deletes or creates the slot's item between
  a macOS writer's lookup and its write or delete
- **THEN** the writer SHALL look the slot up again, with at most five lookups
  in all, and complete on the item that it then finds
- **AND** a delete SHALL report an absent item only when its own lookup found
  none
- **AND** a slot that changes at every lookup SHALL end the operation with a
  failure.

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

#### Scenario: A test drives the platform credential tool

- **WHEN** a test creates, observes or deletes a private Keychain item through
  `/usr/bin/security`, or waits for a child that runs a native operation
- **THEN** its own bound SHALL stop only a hang, with room for a shared CI host
  that is much slower than a quiet one
- **AND** the five-second deadline SHALL remain the bound of AIGW's own
  native credential operations.

## MODIFIED Requirements

### Requirement: Native credential qualification owns real store access

Ordinary source tests SHALL use provider doubles and avoid host credential
stores. A retained-item journey SHALL run only on an explicitly admitted
disposable host, preserve the original item and reader identity, and remain
independent of publisher signing. The scope statement SHALL NOT claim the host
is isolated or disposable. Source coverage or a fresh helper SHALL NOT qualify
retained-item, existing-caller, real-client, or distribution continuity.

#### Scenario: A review runner has no publisher signing identity

- **GIVEN** an explicitly admitted disposable macOS runner
- **WHEN** ordinary native acceptance executes
- **THEN** source tests use provider doubles and do not touch the host Keychain
- **AND** the explicit system-store journey may run without publisher signing
  inputs
- **AND** distribution trust remains a separate release claim.

#### Scenario: Ordinary source verification isolates native credentials

- **WHEN** source verification exercises a native credential worker
- **THEN** it SHALL use the injected provider boundary without accessing the
  operator credential store
- **AND** this result SHALL NOT qualify retained system credentials.

#### Scenario: Explicit Keychain qualification has no disposable-host scope

- **WHEN** macOS native qualification enables system credentials without
  `AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE=ephemeral-host`
- **THEN** it SHALL fail before executing any quality, test, or release command
- **AND** directly selected integration tests SHALL reject that scope before
  native credential access.

#### Scenario: Explicit Keychain qualification uses a disposable host

- **WHEN** macOS native qualification selects both system credentials and the
  disposable-host scope
- **THEN** the existing coverage invocation SHALL include integration tests once
- **AND** selecting full quality SHALL preserve that same inclusion and scope.

#### Scenario: A retained macOS item keeps its reader identity

- **WHEN** a candidate replaces a predecessor that reads an existing Keychain item
- **THEN** predecessor, candidate, and rollback SHALL keep the item's physical
  service and account identity
- **AND** a candidate SHALL reach the item only through its own signing
  identity, never through another program
- **AND** a fresh or replacement helper SHALL NOT establish retained-item access.

## REMOVED Requirements

### Requirement: Native credential operations are bounded

**Reason**: Its bound carried an exception for retained macOS items and a scenario for a concurrent migration, which the product no longer has.

**Migration**: "Every native credential operation is bounded" keeps every other statement of it.
