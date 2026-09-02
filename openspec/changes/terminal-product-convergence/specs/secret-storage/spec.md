## ADDED Requirements

### Requirement: Credential backend state is explicit and portable

AIGW SHALL expose the selected credential backend, its availability, and its
read/write capability without disclosing or retrieving credential values.
Automatic selection SHALL be deterministic for the installation and SHALL NOT
silently cross-read another backend.

#### Scenario: Native credential storage is usable

- **WHEN** the supported platform's native credential service passes a bounded
  non-interactive capability probe
- **THEN** automatic selection records that native backend
- **AND** later commands reuse it without opening an access prompt merely to
  test presence.

#### Scenario: Native credential storage is unavailable

- **WHEN** automatic selection proves the native service unavailable
- **THEN** AIGW selects the declared platform-safe fallback or reports that no
  writable backend is available
- **AND** the result names the exact recovery action without repeated prompts.

#### Scenario: Environment credentials are selected

- **WHEN** the operator explicitly selects environment-backed credentials
- **THEN** setup, sync, check, and client helpers read the documented variables
  consistently
- **AND** every credential mutation reports that the backend is read-only.

### Requirement: Credential availability is scoped to active demand

An Account Token SHALL be required only when an explicit operation activates,
projects, checks, or verifies a Route that selects that Account.

#### Scenario: Catalogue contains unused Accounts

- **WHEN** a reviewed manifest contains Accounts not selected by enabled Routes
- **THEN** their missing Tokens do not block setup, synchronization, status, or
  readiness for the active Routes.
