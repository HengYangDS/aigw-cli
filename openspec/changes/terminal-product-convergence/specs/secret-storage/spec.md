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

### Requirement: Credential compensation is postimage-guarded

A failed credential mutation that became externally visible SHALL restore the
exact preimage only while compensation observes AIGW's own postimage. If
compensation observes another writer's state, AIGW SHALL preserve it and report
that compensation was not applied. Temporary files and partial Tokens SHALL
NOT remain.

#### Scenario: Durable replacement or deletion fails

- **WHEN** AIGW changes a credential slot but cannot prove the change durable
- **THEN** it restores the exact preimage
- **AND** removes every temporary file owned by that attempt.

#### Scenario: The credential changes before compensation

- **WHEN** the current slot no longer equals AIGW's own postimage
- **THEN** compensation refuses to overwrite or delete the newer state
- **AND** the error identifies both the failed operation and the incomplete
  compensation.
