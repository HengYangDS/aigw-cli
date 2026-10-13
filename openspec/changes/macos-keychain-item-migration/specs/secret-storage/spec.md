# Spec Delta

## ADDED Requirements

### Requirement: Retained macOS Keychain items migrate to AIGW ownership

On macOS, a credential operation denied an exact retained item that trusts
`/usr/bin/security` SHALL migrate that item once to an AIGW-owned item at the same
service and Account, without Token entry, access-list change, or authorization UI.
The retained item, a staging item, or both SHALL hold the Token at every step.
Linux and Windows stores authorize the user or session, not the executable, and
SHALL NOT migrate.

#### Scenario: A predecessor item is used after upgrade

- **WHEN** the worker reads, writes, or deletes a slot whose item was written by
  the predecessor's `/usr/bin/security` writer
- **THEN** it SHALL migrate the item and complete the operation without
  interaction
- **AND** a later build with the same signing identifier and team SHALL use the
  item without migration.

#### Scenario: A migration is interrupted

- **WHEN** a worker stops after staging the Token but before the owned item is
  verified
- **THEN** the next read of that slot SHALL complete the migration from the staged
  Token or repeat it
- **AND** the Token SHALL NOT be lost.

#### Scenario: Several workers meet the same retained item

- **WHEN** concurrent workers are denied the same retained item
- **THEN** exactly one SHALL migrate it while the others wait for the owned item
- **AND** none SHALL invoke `/usr/bin/security` on an item that no longer trusts
  it.

#### Scenario: Migration would require interaction

- **WHEN** the item's keychain is locked or the denied item does not trust
  `/usr/bin/security`
- **THEN** the worker SHALL return the denial without invoking
  `/usr/bin/security`, changing an item, or showing UI.

#### Scenario: A portable rollback restores the predecessor reader

- **WHEN** `aigw update --rollback` is about to activate the retained program
- **THEN** AIGW SHALL first convert each configured Account's items back to
  `/usr/bin/security` ownership with the same storage envelope
- **AND** SHALL leave the active program in place if any conversion fails.

#### Scenario: A package manager downgrades AIGW

- **WHEN** Homebrew replaces AIGW with a release earlier than this migration
- **THEN** AIGW SHALL NOT claim credential continuity for that downgrade
- **AND** the predecessor's reader MAY ask macOS to authorize each migrated item.

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
  service and Account identity
- **AND** a candidate SHALL reach a `/usr/bin/security` item only through its
  one-time migration to AIGW ownership
- **AND** a fresh or replacement helper SHALL NOT establish retained-item access.

### Requirement: Native credential operations are bounded

AIGW SHALL run each native credential operation in a private subprocess with a
five-second deadline and bounded cleanup. It SHALL pass only platform identity
required by Keychain, Secret Service, or Credential Manager. A timeout or
authorization failure SHALL terminate the exact operation without retry,
fallback, access-policy change, or backend migration, apart from the one-time
macOS retained-item migration. Creation or timing SHALL NOT prove later
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
  and is not a retained item eligible for migration
- **THEN** the subprocess fails without retrying or returning a Token
- **AND** no credential or access-control state changes
- **AND** any operating-system authorization UI remains an explicit platform
  behavior rather than a suppressed or disproved event.

#### Scenario: The native service does not answer

- **WHEN** the operation reaches its deadline
- **THEN** the parent terminates and reaps its subprocess through bounded cleanup
- **AND** the helper reports the deadline rather than suggesting configuration sync.

### Requirement: Upgrade recovery preserves credential ownership

Candidate, rollback, and re-upgrade SHALL preserve configuration and credential
ownership without ACL change, backend migration, fallback, or helper
substitution. On macOS a successor SHALL migrate retained `/usr/bin/security`
items once, and a portable rollback SHALL convert them back before activating
the predecessor. Complete rollback SHALL restore a compatible program,
configuration, and credential owner.

#### Scenario: Rollback follows a failed candidate

- **WHEN** a candidate fails after replacing the installed program
- **THEN** rollback SHALL restore a compatible reader for the retained item,
  converting a migrated macOS item back before activation
- **AND** the original client invocation SHALL work without fallback, ACL change,
  or a new helper.
