# native-credential-store Specification

## Purpose

Define how AIGW uses the operating system's credential service: what a failure of that service means on
every platform, how availability is observed without interaction, how a service's Tokens are listed by
name, how real stores are qualified, how a secret crosses the process boundary, and the bound on every
native operation.

## Requirements

### Requirement: Native credential service failure has a portable outcome

On supported platforms, automatic resolution SHALL honor an existing backend
choice. Only an unrecorded choice MAY probe native metadata and select the
platform-safe file fallback on failure. The probe SHALL not read Tokens or
claim future read/write permission. Unix fallback SHALL use owner-only storage;
Windows fallback SHALL protect values with current-user DPAPI.

#### Scenario: Headless Linux has no usable credential service

- **WHEN** no backend is recorded and native metadata probing fails on Linux
- **THEN** AIGW selects the secure file backend
- **AND** setup can persist one supplied provider Token

#### Scenario: macOS credential service is unavailable

- **WHEN** no backend is recorded and native metadata probing fails on macOS
- **THEN** AIGW selects the secure file backend without prompting through a client application

#### Scenario: Explicit keyring selection fails closed

- **WHEN** the operator explicitly selects the native credential service
- **AND** that service is unavailable
- **THEN** AIGW reports the backend failure
- **AND** does not silently select the file backend

#### Scenario: Windows automatic selection

- **WHEN** Windows resolves an unrecorded automatic backend
- **THEN** successful native metadata probing SHALL select Credential Manager
- **AND** a failed probe SHALL select the current-user DPAPI file backend
- **AND** only a credential mutation SHALL persist that automatic choice

### Requirement: Native availability observation is non-interactive

AIGW SHALL use value-free native metadata operations to observe credentials on
macOS, Linux, and Windows. Observation SHALL NOT request secret disclosure or
open a credential-access prompt.

#### Scenario: macOS Keychain observation

- **WHEN** AIGW observes a generic-password item on macOS
- **THEN** it queries item metadata without requesting password data

#### Scenario: Linux Secret Service observation

- **WHEN** AIGW observes a Secret Service item on Linux
- **THEN** it searches item attributes without opening a secret session or
  requesting secret bytes

#### Scenario: Linux Secret Service is unavailable

- **WHEN** an explicitly selected or already persisted native backend cannot
  connect to Secret Service while observing a credential on Linux
- **THEN** it reports the connection failure
- **AND** it does not read credential values, select another backend, or open an
  interactive prompt

#### Scenario: Windows Credential Manager observation

- **WHEN** AIGW observes a generic credential on Windows
- **THEN** it uses credential metadata to determine presence without exposing
  the credential blob to AIGW

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

### Requirement: Native credential transport preserves secret boundaries

Tokens SHALL enter the native credential subprocess only through bounded
standard input. Metadata operations SHALL return presence only. Diagnostic
output SHALL never share Token standard output. Encoding SHALL occur once, and
updates SHALL preserve item identity and policy. Every failure SHALL return no
secret and leave credential and access-control state unchanged.

#### Scenario: Native credential output contains diagnostics

- **WHEN** a native provider reports diagnostic output during a credential write
- **THEN** AIGW SHALL keep the diagnostic separate from Token output
- **AND** the diagnostic SHALL NOT be interpreted as credential data.

#### Scenario: A retained released item is consumed after replacement

- **WHEN** the candidate replaces a published predecessor using the same provider
- **THEN** each original client credential command is executed before sync
- **AND** update, rollback and re-upgrade return the retained Token without
  backend migration, helper replacement or configuration drift.

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

### Requirement: A service's Tokens can be listed by name alone

AIGW SHALL list the provider IDs of a service's Tokens from item names or
attributes alone, in the same bounded private subprocess as other native
operations. The listing SHALL read no value, prompt for nothing and pass only
provider IDs across the subprocess boundary. An API that returns values with
the names, as Windows Credential Manager does, SHALL have them zeroed unread
inside the subprocess.

#### Scenario: A store needs a person to list

- **WHEN** a keychain or collection is locked
- **THEN** the listing SHALL name its items or refuse, and SHALL NOT ask for
  access
- **AND** a refusal SHALL be reported as unable to list, never as no Tokens
- **AND** no credential or access-control state SHALL change.
