# secret-storage Specification

## Purpose

Define secure, deterministic Account Token storage that remains usable across
supported operating systems without making a native credential service a
continuous runtime prerequisite.

## Requirements

### Requirement: One backend owns Account Token persistence

AIGW SHALL select exactly one Account Token backend for an invocation and SHALL
NOT read or write another backend as a compatibility fallback.

#### Scenario: Automatic selection remains stable

- **WHEN** AIGW automatically selects a writable backend
- **THEN** it records that selection in AIGW-owned state
- **AND** later invocations reuse the same backend

#### Scenario: Environment storage is explicit and read-only

- **WHEN** the operator selects the environment backend
- **THEN** AIGW reads only documented Account Token environment variables
- **AND** refuses Token writes and deletes

### Requirement: Native credential service failure has a portable outcome

On macOS and Linux, AIGW SHALL use its secure file backend when automatic
selection proves that the native credential service is unavailable. Windows
SHALL retain its native credential manager as the automatic backend.

#### Scenario: Headless Linux has no usable credential service

- **WHEN** automatic selection cannot use the native credential service on Linux
- **THEN** AIGW selects the secure file backend
- **AND** setup can persist one supplied provider Token

#### Scenario: macOS credential service is unavailable

- **WHEN** automatic selection cannot use the native credential service on macOS
- **THEN** AIGW selects the secure file backend without prompting through a client application

#### Scenario: Explicit keyring selection fails closed

- **WHEN** the operator explicitly selects the native credential service
- **AND** that service is unavailable
- **THEN** AIGW reports the backend failure
- **AND** does not silently select the file backend

#### Scenario: Windows automatic selection

- **WHEN** AIGW runs on Windows without an explicit backend
- **THEN** it selects the native credential manager
- **AND** does not create the portable file store

### Requirement: File persistence is owner-only and atomic

The file backend SHALL accept only AIGW-owned regular files and directories
with owner-only permissions, reject links and ambiguous ownership, and replace
Token state atomically within the owning directory.

#### Scenario: Fresh file store

- **WHEN** AIGW first persists a Token in the file backend
- **THEN** the owning directory is mode `0700`
- **AND** the Token file is mode `0600`

#### Scenario: Unsafe storage object

- **WHEN** the storage path or Token file is a symbolic link, has multiple hard links, has unsafe permissions, or is not owned by the current user
- **THEN** AIGW fails before returning or changing Token material

### Requirement: One usable provider is sufficient

Setup SHALL require Tokens only for Accounts selected by the requested Routes;
unselected catalogue Accounts SHALL remain optional.

#### Scenario: Team profile has one available provider

- **WHEN** a team profile declares several Accounts
- **AND** the selected Routes require only one Account with an available Token
- **THEN** setup completes without requesting Tokens for unselected Accounts
