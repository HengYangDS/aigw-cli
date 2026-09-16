## ADDED Requirements

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

### Requirement: macOS credential observations and reads are bounded and noninteractive

The macOS Keychain adapter SHALL execute metadata, read, write and delete
operations in the same AIGW executable's isolated worker. The worker SHALL
disable Keychain interaction before accessing the selected service and slot. Metadata queries
SHALL request neither password bytes nor an item-reference allocation.
The parent SHALL enforce a five-second operation deadline and the existing
bounded process cleanup contract. The child SHALL receive no Token, loader
override, client configuration or fallback-reader selection in its environment.

The adapter SHALL preserve the existing native search-list, service, slot and
stored-value grammar. A denied or locked read SHALL fail without changing access
control, unlocking, migrating a credential or trying another reader. Failure
SHALL return no credential bytes; diagnostics SHALL remain separate from the
client's Token stdout. A metadata success SHALL NOT prove value authorization.

Writes SHALL transport the existing storage envelope through bounded stdin,
never command arguments or environment variables. Creation and reading SHALL
use the same executable identity; updates SHALL preserve item identity and
access policy. Deleting an absent item SHALL be a successful no-op. Each native
operation SHALL own its authorization result; a locked store SHALL NOT imply
that metadata observation or authorized deletion is forbidden. Successful fresh
creation SHALL NOT prove retained-item or cross-release authorization.

#### Scenario: The installed executable creates and rotates a credential

- **WHEN** its native writer creates or updates an authorized exact slot
- **THEN** its native reader returns the written value without interaction
- **AND** mutation emits no Token, including unexpected backend output.

#### Scenario: Exact deletion is repeated

- **WHEN** an authorized deletion removes an item and deletion is requested again
- **THEN** both operations succeed and the exact item remains absent.

#### Scenario: A credential requires authentication interaction

- **WHEN** the exact native item cannot be read without interaction
- **THEN** the worker returns an authorization failure without a password dialog
- **AND** no credential or access-control state changes and no Token is returned.

#### Scenario: The native service does not answer

- **WHEN** the operation reaches its deadline
- **THEN** the parent terminates and reaps its worker through bounded cleanup
- **AND** the helper reports the deadline rather than suggesting configuration sync.

#### Scenario: Another executable created the selected item

- **WHEN** metadata is readable but the item's native policy does not authorize AIGW
- **THEN** observation succeeds and value retrieval fails as authorization denied
- **AND** release or installation acceptance remains open until the actual reader
  identity has independently verified permission; another reader's success is
  not substituted for that proof.
