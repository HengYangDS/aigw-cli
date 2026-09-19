# Spec Delta

## ADDED Requirements

### Requirement: Native credential operations are bounded

AIGW SHALL perform native credential value and metadata operations in a private
five-second worker provided by the active AIGW executable. The parent SHALL use
bounded process cleanup and SHALL pass only the platform identity environment
required by macOS Keychain, Linux Secret Service, or Windows Credential Manager.
Tokens SHALL enter only through bounded standard input; metadata operations SHALL
return presence only and SHALL NOT request secret disclosure. Failures SHALL
return no secret, alter no access policy, migrate nothing, retry nothing, and try
no fallback. Diagnostics SHALL stay off Token standard output. Encoding SHALL
occur once; updates SHALL preserve item identity and policy. Timing and creation
SHALL NOT prove future authorization.

#### Scenario: The installed executable creates and rotates a credential

- **WHEN** its native writer creates or updates an authorized exact slot
- **THEN** its native reader returns the written value without interaction
- **AND** mutation emits no Token, including unexpected backend output.

#### Scenario: Exact deletion is repeated

- **WHEN** an authorized deletion removes an item and deletion is requested again
- **THEN** both operations succeed and the exact item remains absent.

#### Scenario: Credential presence is observed

- **WHEN** AIGW checks an exact native credential slot
- **THEN** the worker returns only present or absent metadata
- **AND** no credential value enters the parent process.

#### Scenario: A credential operation fails or requires authorization

- **WHEN** the exact native item cannot be accessed within the admitted operation
- **THEN** the worker fails without retrying or returning a Token
- **AND** no credential or access-control state changes
- **AND** any operating-system authorization UI remains an explicit platform
  behavior rather than a suppressed or disproved event.

#### Scenario: The native service does not answer

- **WHEN** the operation reaches its deadline
- **THEN** the parent terminates and reaps its worker through bounded cleanup
- **AND** the helper reports the deadline rather than suggesting configuration sync.

#### Scenario: A retained released item is consumed after replacement

- **WHEN** the candidate replaces a published predecessor using the same provider
- **THEN** each original client credential command is executed before sync
- **AND** update, rollback and re-upgrade return the retained Token without
  backend migration, helper replacement or configuration drift.

## REMOVED Requirements

### Requirement: macOS credential value operations are bounded

**Reason:** The macOS-only worker boundary leaves Linux Secret Service and
Windows Credential Manager operations outside the same timeout and cleanup
contract.

**Migration:** All native credential operations now use the cross-platform
`internal/secrets/native` owner through the active AIGW executable.
