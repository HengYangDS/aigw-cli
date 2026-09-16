## ADDED Requirements

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
