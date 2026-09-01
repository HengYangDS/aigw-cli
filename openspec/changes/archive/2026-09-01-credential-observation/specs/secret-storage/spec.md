## ADDED Requirements

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
