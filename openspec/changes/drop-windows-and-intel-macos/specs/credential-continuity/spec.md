## ADDED Requirements

### Requirement: Upgrade evidence preserves each existing caller's credential

Upgrade acceptance SHALL capture each enabled client's command, arguments,
environment, native item, reader, and authorization identity, and SHALL qualify
a successor reader before any client configuration file selects it. Original
invocations of AIGW-owned versioned readers SHALL return the same Token before,
during, and after package replacement without reload; a fresh client or helper
SHALL NOT substitute for existing-caller continuity.

#### Scenario: A proposed adapter survives only CLI-only updates

- **WHEN** unchanged adapter bytes retain item access while the CLI changes
- **THEN** actual adapter replacement, rollback and caller authorization SHALL
  remain unproved until their own retained-item journeys pass
- **AND** preserving a vulnerable old reader SHALL NOT satisfy safe updates.

#### Scenario: A native macOS item needs authorization UI

- **WHEN** an exact selected Keychain item cannot be read without presenting
  authorization UI
- **THEN** the bounded AIGW credential worker SHALL return no Token and SHALL
  NOT open that UI, retry, change the item's ACL, or select another backend
- **AND** a replacement SHALL NOT overwrite an existing item that the selected
  AIGW identity cannot read without interaction
- **AND** an accessible item SHALL retain its logical Token value, while an
  absent item remains distinct from denied access.

#### Scenario: A successor preserves the retained macOS credential identity

- **GIVEN** a retained predecessor command uses an existing provider Token or
  configured balance credential item
- **WHEN** a successor changes the native reader implementation
- **THEN** read, metadata observation, write and delete SHALL use the same
  physical service and account identity without Token enrollment again
- **AND** an authorization denial SHALL block the affected cutover without a
  shadow item, Token copy, ACL mutation, backend fallback, change of the item's
  owner or credential prompt
- **AND** predecessor rotation after rollback SHALL be visible to captured
  readers and the successor before `aigw sync` or re-upgrade
- **AND** exact shadow items from unreleased candidates SHALL remain owned until
  their retained consumers and rollback owners release them.

#### Scenario: A successor creates a macOS native credential item

- **WHEN** AIGW creates a new native Keychain item for a provider Token or
  balance credential
- **THEN** the item SHALL have a readable, nonsecret label naming its purpose
  and provider
- **AND** its service, account, and access identity SHALL remain the credential
  authority; the label SHALL NOT be used to look up or authorize the item.

#### Scenario: A client retains its credential command across an update

- **GIVEN** a client has already loaded a qualified AIGW-owned versioned
  credential command and environment
- **WHEN** the installed AIGW program is replaced without changing the client's
  provider or endpoint
- **THEN** that retained invocation SHALL return the original authorized Token
  before `aigw sync` or a client restart
- **AND** it SHALL keep returning that Token after 0.7.0 writes `aigw.toml`
  and that client's configuration file, because 0.7.0 leaves `config.toml`
  untouched
- **AND** read-only inspection and `aigw check` SHALL accept the unchanged,
  sidecar-owned client configuration file through its intact predecessor
  reader without rewriting it or requiring `aigw sync`
- **AND** a missing, foreign, or drifted predecessor reader SHALL remain invalid
- **AND** updated client configuration files or success from a new helper
  SHALL NOT substitute for that invocation
- **AND** each later captured versioned command SHALL remain callable through
  subsequent rollback and re-upgrade until explicitly retired after proving
  that no consumer can invoke it
- **AND** native credential denial SHALL block production acceptance rather
  than trigger an alternate reader, ACL change, change of the item's owner or
  repeated authorization attempt.

#### Scenario: The package manager temporarily removes the CLI path

- **GIVEN** an update has been admitted for clients retaining original
  AIGW-owned versioned credential commands
- **WHEN** a package manager removes or replaces the CLI executable path
- **THEN** those original invocations before, during, and after that interval
  SHALL return their authorized Token without a missing-executable gap
- **AND** no client reload, Token migration, ACL change, or credential prompt
  SHALL be needed
- **AND** failure SHALL preserve or restore a working original invocation.

#### Scenario: A legacy client still calls the manager-owned CLI link

- **WHEN** a retained caller still uses the executable path a package manager
  will unlink
- **THEN** AIGW SHALL NOT claim the cached caller was migrated by rewriting a
  configuration file or testing a fresh client
- **AND** no release SHALL carry a cutover for that caller: an upgrade from a
  release that wrote such a command is a new setup, and the caller fails until
  its client is set up again.

#### Scenario: The last default Token consumer is withdrawn

- **WHEN** a successful write of client configuration files disables or
  replaces the last client that reads a provider Token through the default
  reader
- **THEN** synchronization and any withdrawal of a client's selection SHALL
  retain the existing credential executable because the current configuration
  cannot prove that cached or rollback callers have stopped invoking it
- **AND** another default consumer, an explicit command, or an incomplete
  rollback of client configuration files SHALL retain the exact executable it
  may still invoke
- **AND** a dry-run SHALL NOT propose automatic deletion based only on the
  current configured consumer count.

#### Scenario: Explicit portable uninstall preserves versioned readers

- **WHEN** an operator runs `aigw self uninstall` on a portable installation
- **THEN** AIGW SHALL withdraw its managed entries from client configuration
  files and remove the selected installation executable and its single
  rollback copy, even if the command runs from a different AIGW binary
- **AND** every existing versioned credential executable and receipt SHALL
  remain byte-for-byte unchanged because a cached caller or another
  installation may still use the same content-addressed reader
- **AND** retaining reader bytes SHALL NOT authorize a client selection
  withdrawn from the selected configuration
- **AND** uninstall SHALL NOT create a reader or receipt that was absent before
  the operation or delete older versioned copies
- **AND** a failed removal SHALL report committed client state with incomplete
  cleanup, not a rolled-back transition or silent success.

#### Scenario: A qualified reader successor is installed

- **GIVEN** a client still invokes an older AIGW-owned reader
- **WHEN** a signed successor is prepared for an installed-product update
- **THEN** AIGW SHALL verify the successor's exact bytes, native credential
  access, selected client configuration file and rollback before switching any
  client
- **AND** SHALL NOT overwrite an executable still used by a retained caller
- **AND** failure SHALL preserve the older command and Token behavior.

#### Scenario: A retired owned credential executable has no consumer

- **WHEN** no configured client, retained caller, rollback target or explicit
  external reference can invoke an owned older executable
- **THEN** only the operator who proves that absence MAY remove that exact
  executable and its owned identity evidence, because AIGW cannot observe cached
  callers
- **AND** AIGW SHALL NOT remove a versioned reader itself, and SHALL preserve
  unknown or foreign content rather than deleting by name prefix or age.

## REMOVED Requirements

### Requirement: Upgrade evidence preserves credential continuity

**Reason**: Its Windows short-name scenario leaves with Windows support, and OpenSpec keeps every scenario of a modified requirement.

**Migration**: "Upgrade evidence preserves each existing caller's credential" keeps every other statement and scenario of it.
