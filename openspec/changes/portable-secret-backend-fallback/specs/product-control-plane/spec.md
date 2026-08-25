## ADDED Requirements

### Requirement: Portable single-backend Token storage

AIGW SHALL allow a token-free team manifest to be imported and SHALL become
usable when any one compatible Provider Account Token is supplied. Missing
Tokens and missing supported clients SHALL remain explicit deferred state.

#### Scenario: Native credential service is unavailable

- **WHEN** the native credential service cannot be used on macOS, Linux, or Windows
- **THEN** automatic selection SHALL pin exactly one platform-protected local fallback
- **AND** setup with one Provider Account Token SHALL not require another Provider Account
- **AND** AIGW SHALL NOT search both native and fallback stores

#### Scenario: Windows fallback persists a Token

- **WHEN** automatic selection falls back on Windows
- **THEN** the stored Token bytes SHALL be protected by current-user DPAPI
- **AND** a subsequent process SHALL resolve the persisted backend and recover the Token
- **AND** explicit `keyring` selection SHALL continue to fail when Credential Manager is unavailable
