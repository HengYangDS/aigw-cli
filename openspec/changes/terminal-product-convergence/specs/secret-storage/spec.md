# Spec Delta

## MODIFIED Requirements

### Requirement: Upgrade evidence preserves credential continuity

Upgrade acceptance SHALL capture each enabled client's command, arguments,
environment, native item, reader, and authorization identity. A successor
reader SHALL be qualified before any client projection selects it. Original
invocations using AIGW-owned versioned readers SHALL return the same Token
before, during, and after package replacement without reload; a fresh client
or helper SHALL NOT substitute for existing-caller continuity. The one-time
published predecessor may retain a package-manager-owned CLI link: its
uncontrolled unlink interval SHALL be bounded, measured, minimized, and
disclosed rather than represented as uninterrupted service.

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
- **AND** an accessible legacy item SHALL retain its logical Token value,
  while an absent item remains distinct from denied access.

#### Scenario: A client retains its credential command across an update

- **GIVEN** a client has already loaded a qualified AIGW-owned versioned
  credential command and environment
- **WHEN** the installed AIGW program is replaced without changing its route
- **THEN** that retained invocation SHALL return the original authorized Token
  before synchronization or client restart
- **AND** read-only inspection and verification SHALL accept the unchanged,
  sidecar-owned projection through its intact predecessor reader without
  rewriting it or requiring synchronization
- **AND** a missing, foreign, or drifted predecessor reader SHALL remain invalid
- **AND** updated projections or success from a new helper SHALL NOT substitute
  for that invocation
- **AND** each later captured versioned command SHALL remain callable through
  subsequent rollback and re-upgrade until explicitly retired after proving
  that no consumer can invoke it
- **AND** native credential denial SHALL block production acceptance rather than
  trigger an alternate reader, ACL change or repeated authorization attempt.

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
- **AND** the one-time cutover SHALL preproject and qualify private readers,
  prefetch the successor, bound and measure the link gap, test captured commands
  immediately, and restore the predecessor on failure
- **AND** a possible cached-link call during that gap SHALL remain an explicit
  residual risk; if that bounded cutover cannot be qualified, it SHALL stop.

#### Scenario: The last default Token consumer is withdrawn

- **WHEN** a successful projection disables or replaces the last default
  Account-Token Client Binding
- **THEN** ordinary synchronization and Client Binding withdrawal SHALL retain
  the existing credential executable because the current configuration cannot
  prove that cached or rollback callers have stopped invoking it
- **AND** another default consumer, an explicit command, or an incomplete
  projection rollback SHALL retain the exact executable it may still invoke
- **AND** a dry-run SHALL NOT propose automatic deletion based only on the
  current configured consumer count.

#### Scenario: Explicit portable uninstall preserves versioned readers

- **WHEN** an operator explicitly uninstalls a portable AIGW installation
- **THEN** AIGW SHALL withdraw its managed projections and remove the selected
  installation executable and its single rollback copy, even if the command
  runs from a different AIGW binary
- **AND** every existing versioned credential executable and receipt SHALL
  remain byte-for-byte unchanged because a cached caller or another
  installation may still use the same content-addressed reader
- **AND** retaining reader bytes SHALL NOT authorize a Client Binding withdrawn
  from the selected configuration
- **AND** uninstall SHALL NOT create a reader or receipt that was absent before
  the operation or delete older versioned copies
- **AND** a failed removal SHALL report committed client state with incomplete
  cleanup, not a rolled-back transition or silent success.

#### Scenario: A qualified reader successor is installed

- **GIVEN** a client still invokes an older AIGW-owned reader
- **WHEN** a signed successor is prepared for an installed-product update
- **THEN** AIGW SHALL verify the successor's exact bytes, native credential
  access, selected projection and rollback before switching any client
- **AND** SHALL NOT overwrite an executable still used by a retained caller
- **AND** failure SHALL preserve the older command and Token behavior.

#### Scenario: A retired owned credential executable has no consumer

- **WHEN** no configured client, retained caller, rollback target or explicit
  external binding can invoke an owned older executable
- **THEN** AIGW SHALL remove only that exact executable and its owned identity
  evidence
- **AND** SHALL preserve unknown or foreign content rather than deleting by
  name prefix or age.
