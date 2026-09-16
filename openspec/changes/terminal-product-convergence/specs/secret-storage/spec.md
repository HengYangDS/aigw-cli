## ADDED Requirements

### Requirement: Native credential qualification owns real store access

Ordinary source tests SHALL use provider doubles and avoid the host Keychain.
The retained-system-credential journey SHALL run only on an explicitly admitted
disposable host, use the published go-keyring `/usr/bin/security` provider, and
remain independent of publisher signing credentials. Source coverage SHALL NOT
be reported as retained-item, real-client or distribution proof.

#### Scenario: A review runner has no publisher signing identity

- **GIVEN** an explicitly admitted disposable macOS runner
- **WHEN** ordinary native acceptance executes
- **THEN** source tests use provider doubles and do not touch the host Keychain
- **AND** the explicit system-store journey may run without publisher signing
  inputs
- **AND** distribution trust remains a separate release claim.

### Requirement: Explicit host credential policy survives client projection

A local Adapter MAY select one trusted absolute credential executable with
`credential_command`. Its invocation SHALL retain the existing
`credential <client> <projection-fingerprint>` contract. The setting SHALL NOT
change the native `aigw credential` implementation, select another secret backend,
enter a team manifest, or grant native credential access.

Sync, program upgrade and explicit Adapter re-enablement SHALL preserve this
setting. Disable SHALL withdraw the client projection while retaining disabled
host policy; sync SHALL NOT implicitly re-enable it. Full uninstall SHALL remove
Adapter policy without deleting the external executable or its credentials.
Client verification SHALL consume the synchronized helper through the existing
bounded native client runner, without a redundant native-store read. Unknown
credential-bearing diagnostics SHALL be suppressed. A successful client exit
without the expected verification marker SHALL remain a verification failure.
Readiness, route selection and re-enablement SHALL NOT require a duplicate Token
in AIGW's store. A local check SHALL report projection readiness, not claim
external credential retrieval or endpoint authentication has succeeded.

#### Scenario: A trusted external reader supplies the client credential

- **GIVEN** an enabled Adapter with an explicit absolute credential executable
- **WHEN** sync, program upgrade and live client verification execute
- **THEN** sync and upgrade preserve the helper and do not read a Token
- **AND** verification uses the synchronized native client, not a fallback reader
- **AND** disabled policy remains disabled until explicit re-enablement
- **AND** an older binary that cannot parse this policy is rejected before rollback.

### Requirement: macOS credential value operations are bounded

The macOS Keychain adapter SHALL execute read, write and delete operations in a
private AIGW worker backed by go-keyring's `/usr/bin/security` provider. The
parent SHALL enforce a five-second deadline and the existing bounded process
cleanup contract. Writes SHALL pass the logical Token through bounded standard
input; the child SHALL receive no Token, loader override, client configuration
or fallback-reader selection in its environment. Metadata observation SHALL use
a value-free query that does not request password bytes.

The adapter SHALL preserve the existing search-list, service, slot and stored-
value grammar. A failed or timed-out read SHALL return no credential bytes,
change no access control, migrate no credential and try no alternate backend.
Diagnostics SHALL remain separate from Token stdout. A metadata success SHALL
NOT prove value authorization. The process deadline SHALL NOT be represented as
a guarantee that macOS cannot display authorization UI.

go-keyring SHALL apply its storage encoding exactly once. Updates SHALL preserve
item identity and access policy. Deleting an absent item SHALL be a successful
no-op. Successful fresh creation SHALL NOT prove retained-item or cross-release
authorization.

#### Scenario: The installed executable creates and rotates a credential

- **WHEN** its native writer creates or updates an authorized exact slot
- **THEN** its native reader returns the written value without interaction
- **AND** mutation emits no Token, including unexpected backend output.

#### Scenario: Exact deletion is repeated

- **WHEN** an authorized deletion removes an item and deletion is requested again
- **THEN** both operations succeed and the exact item remains absent.

#### Scenario: A credential read fails or requires authorization

- **WHEN** the exact native item cannot be read within the admitted operation
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
