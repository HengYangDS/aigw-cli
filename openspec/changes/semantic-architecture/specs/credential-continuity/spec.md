## Purpose

Define how Account credentials stay readable and owned across upgrade,
migration, rollback and configuration evolution.

## ADDED Requirements

### Requirement: Upgrade evidence preserves credential continuity

Upgrade acceptance SHALL capture each enabled client's command, arguments,
environment, native item, reader, and authorization identity, and SHALL qualify
a successor reader before any client projection selects it. Original
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
- **AND** it does not qualify under "Retained macOS Keychain items migrate to
  AIGW ownership"
- **THEN** the bounded AIGW credential worker SHALL return no Token and SHALL
  NOT open that UI, retry, change the item's ACL, or select another backend
- **AND** a replacement SHALL NOT overwrite an existing item that the selected
  AIGW identity cannot read without interaction
- **AND** an accessible legacy item SHALL retain its logical Token value,
  while an absent item remains distinct from denied access.

#### Scenario: A successor preserves the retained macOS credential identity

- **GIVEN** a retained predecessor command uses an existing Account Token or
  configured provider-diagnostic item
- **WHEN** a successor changes the native reader implementation
- **THEN** read, metadata observation, write and delete SHALL use the same
  physical service and Account identity without Token enrollment again
- **AND** only an exact retained item admitted by "Retained macOS Keychain items
  migrate to AIGW ownership" MAY change credential ownership through that
  one-time migration
- **AND** all other authorization denials SHALL block the affected cutover
  without a shadow item, Token copy, ACL mutation, backend fallback or credential
  prompt
- **AND** predecessor rotation after rollback SHALL be visible to captured
  readers and the successor before projection synchronization or re-upgrade
- **AND** exact shadow items from unreleased candidates SHALL remain owned until
  their retained consumers and rollback owners release them.

#### Scenario: A successor creates a macOS native credential item

- **WHEN** AIGW creates a new native Keychain item for an Account Token or
  provider-diagnostic credential
- **THEN** the item SHALL have a readable, nonsecret label naming its purpose
  and logical Account
- **AND** its service, account, and access identity SHALL remain the credential
  authority; the label SHALL NOT be used to look up or authorize the item.

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
- **AND** native credential denial outside the admitted macOS retained-item
  migration SHALL block production acceptance rather than trigger an alternate
  reader, ACL change or repeated authorization attempt.

#### Scenario: A Windows credential reader exceeds the native shell path limit

- **GIVEN** an owned versioned reader has a canonical path longer than the
  native Windows shell can execute
- **WHEN** AIGW prepares its shell-based credential invocation
- **THEN** it SHALL use only a Windows-provided short name whose existing
  ancestor identifies the same native file object
- **AND** command spelling SHALL remain stable before and after reader
  preparation; parsing SHALL preserve that spelling while inspection proves
  the same native versioned namespace
- **AND** it SHALL NOT create an alias, launcher, junction, credential helper,
  alternate store, or native short-name policy
- **AND** an unavailable, ambiguous, unsafe, or insufficient native name SHALL
  fail before configuration or client projections are committed.

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
  prefetch the successor, bound, measure and minimize the link gap, test
  captured commands immediately, and restore the predecessor on failure
- **AND** a possible cached-link call during that gap SHALL remain an explicit
  residual risk, never represented as uninterrupted service; if that bounded
  cutover cannot be qualified, it SHALL stop.

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
- **THEN** only the operator who proves that absence MAY remove that exact
  executable and its owned identity evidence, because AIGW cannot observe cached
  callers
- **AND** AIGW SHALL NOT remove a versioned reader itself, and SHALL preserve
  unknown or foreign content rather than deleting by name prefix or age.

### Requirement: Upgrade recovery preserves credential ownership

Candidate, rollback, and re-upgrade SHALL preserve configuration and credential
ownership without ACL change, backend migration, fallback, or helper
substitution. On macOS the installed successor SHALL migrate retained
`/usr/bin/security` items once, and a rollback SHALL return them to the retained
program's reader. Complete rollback SHALL restore a compatible program,
configuration, and credential owner.

#### Scenario: Rollback follows a failed candidate

- **WHEN** a candidate fails after replacing the installed program
- **THEN** rollback SHALL restore a compatible reader for the retained item,
  converting a migrated macOS item back before activation
- **AND** the original client invocation SHALL work without fallback, ACL change,
  or a new helper.

### Requirement: Retained macOS Keychain items migrate to AIGW ownership

On macOS, when the installed program's credential operation is denied an exact
retained item that trusts `/usr/bin/security`, it SHALL migrate that item once to
an AIGW-owned item at the same service and Account, without Token entry,
access-list change, or authorization UI, keeping the Token in the retained item,
a staging item, or both at every step. No other AIGW program SHALL change an
item's owner. Linux and Windows stores authorize the user or session and SHALL
NOT migrate.

#### Scenario: A predecessor item is used after upgrade

- **WHEN** the installed program's worker reads, writes, or deletes a slot whose
  item was written by the predecessor's `/usr/bin/security` writer
- **THEN** it SHALL migrate the item and complete the operation without
  interaction
- **AND** a later build with the same signing identifier and team SHALL use the
  item without migration.

#### Scenario: Another AIGW program meets a retained item

- **GIVEN** the installed program is an executable that Homebrew's receipt owns,
  or a portable one beside the predecessor AIGW retained for its rollback
- **WHEN** a versioned reader copy, an extracted release or a development build
  reads a slot whose item still trusts `/usr/bin/security`, or whose migration
  the installed program left staged
- **THEN** it SHALL return the Token through that tool or from the staging item
  without migrating the item or completing the migration, because it cannot
  tell whether its installation still runs the predecessor
- **AND** SHALL NOT replace the retained item or convert any item for a rollback
- **AND** the predecessor and every cached successor command SHALL keep reading
  the same item without a prompt
- **AND** the worker SHALL learn its role from its own executable's installation
  evidence, not from a flag or marker its caller could omit.

#### Scenario: A migration is interrupted

- **WHEN** a worker stops after staging the Token but before the owned item is
  verified
- **THEN** the installed program's next read of that slot SHALL complete the
  migration from the staged Token or repeat it
- **AND** the Token SHALL NOT be lost.

#### Scenario: Several workers meet the same retained item

- **WHEN** concurrent workers are denied the same retained item
- **THEN** exactly one SHALL migrate it while the others wait for the owned item
- **AND** none SHALL invoke `/usr/bin/security` on an item that no longer trusts
  it.

#### Scenario: Migration would require interaction

- **WHEN** the item's keychain is locked or the denied item does not trust
  `/usr/bin/security`
- **THEN** the worker SHALL return the denial without invoking
  `/usr/bin/security`, changing an item, or showing UI.

#### Scenario: A portable rollback restores the predecessor reader

- **WHEN** `aigw update --rollback` has admitted the retained program and the
  current configuration, and that program predates AIGW-owned items
- **THEN** AIGW SHALL convert each configured Account's items back to
  `/usr/bin/security` ownership with the same storage envelope before activating
  that program
- **AND** a refused admission SHALL change no item
- **AND** if a conversion or the activation fails, AIGW SHALL return the
  converted items to the active program and leave that program in place
- **AND** a retained program that reads AIGW-owned items SHALL need no
  conversion.

#### Scenario: A package manager downgrades AIGW

- **WHEN** `aigw update --rollback` runs on a Homebrew installation
- **THEN** AIGW SHALL convert each configured Account's items back without
  changing Homebrew-owned files and direct the operator to Homebrew for the
  earlier release
- **AND** if a conversion fails, AIGW SHALL return the converted items and report
  that the installed program is unchanged
- **AND** if the configuration that names those Accounts cannot be read or
  decoded, AIGW SHALL change no item and SHALL NOT report the credentials ready
- **AND** if the operator keeps the release, its next Token use SHALL migrate the
  items again
- **AND** without that preparation AIGW SHALL NOT claim credential continuity,
  and the predecessor's reader MAY ask macOS to authorize each migrated item.

### Requirement: One command verifies each credential executable once

Within one AIGW command, credential reader preparation SHALL read the running
program once to verify its content identity, and each versioned reader once
before client projection and once after it, with memory bounded independently
of the program's size. It SHALL install only program bytes whose digest equals
that identity. No identity SHALL be reused by a later command or keyed by file
metadata.

#### Scenario: A synchronization reuses its verified reader

- **WHEN** one command derives, verifies and projects an existing versioned
  credential reader
- **THEN** it SHALL read the program once and the reader twice
- **AND** later steps of the same command SHALL use those verified identities.

#### Scenario: A command installs the reader

- **WHEN** the versioned reader is absent
- **THEN** AIGW MAY read the program once more to copy it
- **AND** it SHALL refuse the copy unless its digest equals the verified one.

#### Scenario: The program changes after verification

- **WHEN** the running program's bytes change after the command verified them
  and the reader must be installed
- **THEN** AIGW SHALL refuse the installation and leave no reader state.

#### Scenario: A reader changes during projection

- **WHEN** a verified reader is removed, replaced or rewritten in place before
  projection completes, even with an unchanged size and timestamp
- **THEN** AIGW SHALL refuse it and roll back the projection.

#### Scenario: A later command meets changed bytes

- **WHEN** a program or reader changed after an earlier command verified it,
  even with an unchanged size and timestamp
- **THEN** the later command SHALL verify the current bytes by content
- **AND** it SHALL refuse a reader whose bytes no longer match its receipt.

### Requirement: Configuration evolution preserves retained credential readers

AIGW SHALL write the canonical configuration only in a form that every
published credential reader of the same schema version accepts. A Route
capability outside the vocabulary published with that schema SHALL be stored
in a separate configuration component that credential readers do not read,
while the full program SHALL read both.

#### Scenario: A Route declares a capability that earlier readers do not know

- **WHEN** a Route declares a capability outside the published schema
  vocabulary and AIGW saves the configuration
- **THEN** the canonical configuration SHALL contain only published-vocabulary
  capabilities
- **AND** every published credential reader of that schema SHALL return the
  projected Token from it
- **AND** AIGW SHALL still report and project the declared capability.

#### Scenario: The capability component is damaged

- **WHEN** the separate capability component cannot be parsed or validated
- **THEN** a credential reader SHALL still return the projected Token
- **AND** other configuration reads SHALL fail without changing stored files.

#### Scenario: Recovery restores a capability with its configuration

- **WHEN** AIGW compensates a configuration write, restores a backup or reads
  a verified checkpoint
- **THEN** each restored configuration SHALL declare the stored capabilities
  that applied to it
- **AND** a verified checkpoint SHALL contain only published-vocabulary
  capabilities.

### Requirement: Stored capabilities follow their recorded Route

AIGW SHALL accept a capability outside the published schema vocabulary at its
earlier location in the canonical configuration and SHALL move it to the
separate component at its next configuration write. A stored capability SHALL
apply only while its Route keeps the Account, endpoint, upstream model and
interface it was recorded for.

#### Scenario: An earlier release wrote the capability into the canonical configuration

- **WHEN** the canonical configuration declares a capability outside the
  published schema vocabulary
- **THEN** AIGW SHALL accept the configuration with that capability declared
- **AND** its next configuration write SHALL remove the capability from the
  canonical configuration without changing what the Route declares.

#### Scenario: The recorded Route changes

- **WHEN** the Route of a stored capability no longer exists or no longer
  matches the Account, endpoint, upstream model or interface it was recorded
  for
- **THEN** the capability SHALL NOT apply
- **AND** the next configuration write SHALL drop it.

### Requirement: Credential configuration failures name the reader and stage

When a credential reader cannot read, parse or validate the configuration, it
SHALL return no Token and SHALL report its own AIGW version and the failed
stage. A read failure SHALL also name the operating-system operation and error.
The report SHALL NOT contain a path, configuration content or a Token. After a
parse or validation failure, its recommended action SHALL name synchronization
with the installed AIGW and a client reload.

#### Scenario: A reader cannot validate a newer configuration

- **WHEN** a credential reader fails to validate the configuration
- **THEN** its diagnostic SHALL name the reader's AIGW version and the validate
  stage
- **AND** standard output SHALL stay empty
- **AND** the recommended action SHALL name `aigw sync` and a client reload.

#### Scenario: The configuration cannot be opened

- **WHEN** reading the configuration fails with an operating-system error
- **THEN** the diagnostic SHALL name the read stage, the failed operation and
  the error
- **AND** it SHALL NOT contain the configuration path.

### Requirement: Recovery advice follows the selected credential backend

When AIGW recommends providing or replacing an Account Token or a
provider-diagnostic credential, the recommendation SHALL be one that the
selected credential backend can perform. A writable backend SHALL name the AIGW
command that stores the credential. The read-only environment backend SHALL
name the exact environment variables to set and SHALL NOT recommend a command
that it refuses.

#### Scenario: The endpoint rejects an environment Token

- **WHEN** `aigw check` receives HTTP 401 for an Account whose Token comes from
  the environment backend
- **THEN** its human and JSON output SHALL name that Account's Token variable
- **AND** SHALL NOT recommend `aigw rotate`.

#### Scenario: Environment diagnostic credentials are absent

- **WHEN** `aigw account balance` finds no provider-diagnostic credential in the
  environment backend
- **THEN** it SHALL name both diagnostic variables for that Account
- **AND** SHALL NOT recommend `aigw account diagnostics enable`.

#### Scenario: A writable backend needs a credential

- **WHEN** the same failures occur with a writable backend
- **THEN** the recommendation SHALL remain `aigw rotate <account>` or
  `aigw account diagnostics enable <account>`.
