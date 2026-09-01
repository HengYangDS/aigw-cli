# secret-storage Specification

## Purpose

Define secure, deterministic Account Token storage that remains usable across
supported operating systems without making a native credential service a
continuous runtime prerequisite.

## Requirements

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

### Requirement: One backend owns credential persistence

AIGW SHALL select exactly one credential backend for an invocation. The same
backend SHALL own API Tokens and provider-diagnostic credentials in distinct
typed slots, and AIGW SHALL NOT read or write another backend as a
compatibility fallback.

#### Scenario: Automatic selection remains stable

- **WHEN** AIGW automatically selects a writable backend
- **THEN** it records that selection in AIGW-owned state
- **AND** later invocations reuse the same backend for every credential purpose

#### Scenario: Credential purposes remain isolated

- **WHEN** one Account has an API Token and a provider-diagnostic credential
- **THEN** each value is read, replaced, and deleted through its own typed slot
- **AND** an operation on one purpose SHALL NOT change the other

#### Scenario: Environment storage is explicit and read-only

- **WHEN** the operator selects the environment backend
- **THEN** AIGW reads only documented Account credential environment variables
- **AND** refuses credential writes and deletes

#### Scenario: Environment Account names cannot collide

- **WHEN** two valid Account IDs differ by dot, dash, or underscore
- **THEN** AIGW derives distinct deterministic environment variable names
- **AND** the mapping identifies the original lowercase Account ID without an
  ambiguous normalization rule

#### Scenario: Diagnostic environment credential is incomplete

- **WHEN** only the diagnostic system token or only the diagnostic user ID is
  present for an Account
- **THEN** AIGW treats the provider-diagnostic credential as unavailable
- **AND** never substitutes the Account API Token or another backend

### Requirement: Credential availability is observable without disclosure

AIGW SHALL observe whether an Account credential is present without retrieving
its secret value. The observation SHALL distinguish present, absent, and
backend failure; it SHALL NOT report a backend failure as absence.

#### Scenario: Read-only journey observes a present credential

- **WHEN** status, setup, sync, profile, route, adapter, manifest, doctor, or
  catalogue logic needs only credential availability
- **THEN** AIGW queries credential metadata without retrieving the value
- **AND** does not initiate authentication or credential mutation

#### Scenario: Credential backend observation fails

- **WHEN** the selected credential backend cannot determine availability
- **THEN** AIGW reports the backend failure
- **AND** does not describe the credential as missing

#### Scenario: Authentication needs the credential value

- **WHEN** AIGW performs an explicit provider or client authentication action
- **THEN** it retrieves the credential value from the same selected backend
- **AND** no availability cache or alternate backend becomes authoritative

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

#### Scenario: Windows Credential Manager observation

- **WHEN** AIGW observes a generic credential on Windows
- **THEN** it uses credential metadata to determine presence without exposing
  the credential blob to AIGW
