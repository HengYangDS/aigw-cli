## ADDED Requirements

### Requirement: Account Token availability is scoped to active demand

An Account Token SHALL be required only for an explicit operation on an active
Account-Token Route. Client-native authentication SHALL remain independent.
Readiness SHALL admit the selected executable and projection before reading a
Token or probing an endpoint. An unready projection SHALL retain its human and
JSON recovery action; an unobserved credential SHALL NOT be classified as absent.

#### Scenario: A selected client's projection is not ready

- **WHEN** `aigw check` observes a missing executable, missing target or invalid
  projection for an enabled client
- **THEN** it reports that client's projection problem and recovery action
- **AND** it reads no Token value and makes no authenticated request for that
  client, even if its Token is missing or unreadable.

#### Scenario: A selected client's projection is ready

- **WHEN** the enabled client's projection passes local checks
- **THEN** an Account-authenticated Route reads its Token once before endpoint
  diagnostics, retaining credential failure details when the read fails
- **AND** client-owned authentication reads no AIGW credential or endpoint.

#### Scenario: Catalog contains unused Accounts

- **WHEN** a reviewed manifest contains Accounts not selected by enabled Routes
- **THEN** their missing Tokens do not block setup, synchronization, status, or
  readiness for the active Routes.

## MODIFIED Requirements

### Requirement: One usable provider is sufficient

Setup SHALL require Tokens only for Accounts selected by the requested Routes;
unselected catalog Accounts SHALL remain optional.

#### Scenario: Team manifest has one available provider

- **WHEN** a team manifest declares several Accounts
- **AND** the selected Routes require only one Account with an available Token
- **THEN** setup completes without requesting Tokens for unselected Accounts

### Requirement: Credential availability is observable without disclosure

AIGW SHALL observe whether an Account credential is present without retrieving
its secret value. The observation SHALL distinguish present, absent, and
backend failure; it SHALL NOT report a backend failure as absence.

#### Scenario: Read-only journey observes a present credential

- **WHEN** status, setup, sync, route, adapter, manifest, doctor, or
  catalog logic needs only credential availability
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

## REMOVED Requirements

### Requirement: Credential availability is scoped to active demand

**Reason**: Its title or scenario names spell catalog as the glossary does, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Account Token availability is scoped to active demand" in this capability, with every scenario.

### Requirement: Upgrade evidence preserves credential continuity

**Reason**: The `credential-continuity` capability owns this requirement's subject.

**Migration**: Read it in `credential-continuity`, with every scenario.

### Requirement: Upgrade recovery preserves credential ownership

**Reason**: The `credential-continuity` capability owns this requirement's subject.

**Migration**: Read it in `credential-continuity`, with every scenario.

### Requirement: Retained macOS Keychain items migrate to AIGW ownership

**Reason**: The `credential-continuity` capability owns this requirement's subject.

**Migration**: Read it in `credential-continuity`, with every scenario.

### Requirement: One command verifies each credential executable once

**Reason**: The `credential-continuity` capability owns this requirement's subject.

**Migration**: Read it in `credential-continuity`, with every scenario.

### Requirement: Configuration evolution preserves retained credential readers

**Reason**: The `credential-continuity` capability owns this requirement's subject.

**Migration**: Read it in `credential-continuity`, with every scenario.

### Requirement: Stored capabilities follow their recorded Route

**Reason**: The `credential-continuity` capability owns this requirement's subject.

**Migration**: Read it in `credential-continuity`, with every scenario.

### Requirement: Credential configuration failures name the reader and stage

**Reason**: The `credential-continuity` capability owns this requirement's subject.

**Migration**: Read it in `credential-continuity`, with every scenario.

### Requirement: Recovery advice follows the selected credential backend

**Reason**: The `credential-continuity` capability owns this requirement's subject.

**Migration**: Read it in `credential-continuity`, with every scenario.
