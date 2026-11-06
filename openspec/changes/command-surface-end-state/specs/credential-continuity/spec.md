## ADDED Requirements

### Requirement: The credential reader decodes a frozen view

The credential reader SHALL decode from `aigw.toml` only each provider's ID and
endpoints, the protocol keys of its models, and each client's enabled state and
authentication. It SHALL ignore `version`, every other field and unknown
protocol keys, refuse a client whose authentication kind it does not know, and
match fingerprints exactly. A release that changes one of these fields SHALL
store its configuration under a new file name.

#### Scenario: A later release adds fields and a version

- **GIVEN** `aigw.toml` holds fields this reader does not know and
  `version = 99`
- **WHEN** a client calls the reader with a fingerprint that matches one of its
  providers' endpoints
- **THEN** the reader SHALL return that provider's Token
- **AND** its standard output SHALL contain only the Token.

#### Scenario: A field outside the view is malformed

- **WHEN** a field outside the view holds a value the full program would
  reject, such as a label of the wrong type or an unknown capability
- **THEN** the reader SHALL still return the matching Token
- **AND** a document that is not valid TOML SHALL fail at the parse stage, as
  "Credential configuration failures name the reader and stage" requires.

#### Scenario: A later release adds a protocol

- **WHEN** a provider's endpoints or a model's protocols hold a protocol key
  this reader does not know
- **THEN** the reader SHALL ignore that key
- **AND** it SHALL still return the Token for a fingerprint that matches an
  endpoint of a protocol it knows.

#### Scenario: Fingerprints survive the import

- **GIVEN** AIGW 0.6.1 fingerprinted a client's connection to a provider
  endpoint
- **WHEN** `aigw sync` imports that configuration into `aigw.toml`
- **THEN** the reader's fingerprint for the same client, provider ID and
  upstream endpoint SHALL equal 0.6.1's
- **AND** the protocol names `responses` and `chat` SHALL NOT change it.

#### Scenario: The reader refuses a client it cannot serve

- **WHEN** the named client is disabled, uses its own authentication or an
  authentication kind this reader does not know, or no provider endpoint
  matches the fingerprint
- **THEN** the reader SHALL return no Token and SHALL read no credential.

#### Scenario: A later release changes a field in the view

- **WHEN** a release changes the name, type or meaning of a field in the view
- **THEN** it SHALL write its configuration under a new file name
- **AND** it SHALL leave `aigw.toml` readable for readers in running sessions.

### Requirement: Upgrading imports a schema 6 configuration once

`aigw sync` SHALL import a schema 6 `config.toml` and its component files into
`aigw.toml` when only the old files exist, installing the new credential
reader and pointing client configuration files at it in one transaction. No
command SHALL write or delete the old files, and `aigw.toml` SHALL win whenever
both exist. Until the import, other commands that need the configuration SHALL
refuse and name `aigw sync`, except `aigw self`, shell completion and
`aigw config path`.

#### Scenario: The first sync after the upgrade imports the configuration

- **GIVEN** a schema 6 `config.toml` with its component files, client
  configuration files that match it, and no `aigw.toml`
- **WHEN** `aigw sync` runs
- **THEN** it SHALL write `aigw.toml` with the providers, models, families,
  recommendations and client settings, including forwarding, retry budgets and
  capabilities, that the old files declare, in their display order and naming
  protocols `anthropic`, `responses` and `chat`
- **AND** each client configuration file SHALL change only in the credential
  reader's path
- **AND** `config.toml`, its component files and every Token item SHALL stay
  unchanged
- **AND** a credential command that a running session loaded before the sync
  SHALL keep returning its Token from those files.

#### Scenario: An import is previewed

- **WHEN** `aigw sync --dry-run` runs while an import is pending
- **THEN** it SHALL report the import and each client configuration file it
  would rewrite
- **AND** it SHALL write no `aigw.toml`, credential reader or client file and
  read no Token.

#### Scenario: Both files exist

- **GIVEN** `aigw.toml` and `config.toml` both exist, as after a program
  rollback and a forward upgrade
- **WHEN** `aigw sync` or any other command loads the configuration
- **THEN** it SHALL load only `aigw.toml` and import nothing
- **AND** changes made after the first import SHALL survive.

#### Scenario: Another command meets a pending import

- **GIVEN** `config.toml` exists and `aigw.toml` does not
- **WHEN** a command that needs the configuration runs, other than `aigw sync`,
  an `aigw self` command, shell completion or `aigw config path`
- **THEN** it SHALL refuse, name `aigw sync`, and change nothing.

#### Scenario: Some commands still run while an import is pending

- **WHEN** an `aigw self` command, `aigw version`, `aigw completion`,
  `aigw config path` or shell completion runs while an import is pending
- **THEN** it SHALL run without importing anything
- **AND** completion SHALL suggest nothing.

#### Scenario: A program rollback runs while an import is pending

- **WHEN** `aigw self rollback` runs while an import is pending
- **THEN** it SHALL read `config.toml` in memory to name the Token items
- **AND** it SHALL write no configuration file and SHALL NOT refuse because the
  import is pending.

#### Scenario: The old configuration predates schema 6

- **WHEN** `aigw sync` finds a `config.toml` in a schema older than 6
- **THEN** it SHALL refuse, name a release that migrates it to schema 6, and
  write nothing.

#### Scenario: The import fails

- **WHEN** a step of the import transaction fails
- **THEN** `aigw.toml` SHALL remain absent and every client configuration file
  SHALL keep its previous bytes
- **AND** `config.toml` and its component files SHALL stay untouched.

#### Scenario: A program rollback stages the file the previous program reads

- **WHEN** `aigw self rollback` checks whether the previous program can read
  its configuration
- **THEN** the isolated copy SHALL hold `config.toml` and its component files as
  they are for a program older than 0.7.0, and `aigw.toml` otherwise
- **AND** `aigw.toml` SHALL stay unchanged, so its changes return on a later
  forward upgrade
- **AND** when both files exist, the result SHALL say that changes made since
  the import are invisible to a release older than 0.7.0.

## MODIFIED Requirements

### Requirement: Upgrade evidence preserves credential continuity

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
- **AND** it does not qualify under "Retained macOS Keychain items migrate to
  AIGW ownership"
- **THEN** the bounded AIGW credential worker SHALL return no Token and SHALL
  NOT open that UI, retry, change the item's ACL, or select another backend
- **AND** a replacement SHALL NOT overwrite an existing item that the selected
  AIGW identity cannot read without interaction
- **AND** an accessible legacy item SHALL retain its logical Token value,
  while an absent item remains distinct from denied access.

#### Scenario: A successor preserves the retained macOS credential identity

- **GIVEN** a retained predecessor command uses an existing provider Token or
  configured balance credential item
- **WHEN** a successor changes the native reader implementation
- **THEN** read, metadata observation, write and delete SHALL use the same
  physical service and account identity without Token enrollment again
- **AND** only an exact retained item admitted by "Retained macOS Keychain items
  migrate to AIGW ownership" MAY change credential ownership through that
  one-time migration
- **AND** all other authorization denials SHALL block the affected cutover
  without a shadow item, Token copy, ACL mutation, backend fallback or credential
  prompt
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
- **AND** it SHALL keep returning that Token after the 0.7.0 `aigw sync`
  imports the configuration, because the import leaves `config.toml` untouched
- **AND** when no import is pending, read-only inspection and `aigw check`
  SHALL accept the unchanged, sidecar-owned client configuration file through
  its intact predecessor reader without rewriting it or requiring `aigw sync`
- **AND** a missing, foreign, or drifted predecessor reader SHALL remain invalid
- **AND** updated client configuration files or success from a new helper
  SHALL NOT substitute for that invocation
- **AND** each later captured versioned command SHALL remain callable through
  subsequent rollback and re-upgrade until explicitly retired after proving
  that no consumer can invoke it
- **AND** a client session started before the 0.7.0 `aigw sync` and still
  running after the 0.8.0 `aigw sync` SHALL get no Token from its reader and
  must restart, because 0.8.0 deletes the files that reader reads
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
  fail before configuration or client configuration files are committed.

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
- **AND** the one-time cutover SHALL first point client configuration files at
  qualified private readers, prefetch the successor, bound, measure and
  minimize the link gap, test captured commands immediately, and restore the
  predecessor on failure
- **AND** a possible cached-link call during that gap SHALL remain an explicit
  residual risk, never represented as uninterrupted service; if that bounded
  cutover cannot be qualified, it SHALL stop.

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

### Requirement: Retained macOS Keychain items migrate to AIGW ownership

On macOS, when the installed program's credential operation is denied an exact
retained item that trusts `/usr/bin/security`, it SHALL migrate that item once to
an AIGW-owned item at the same service and account, without Token entry,
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

- **WHEN** `aigw self rollback` has checked the retained program against the
  configuration it reads, and that program predates AIGW-owned items
- **THEN** AIGW SHALL convert each configured provider's items back to
  `/usr/bin/security` ownership with the same storage envelope before activating
  that program
- **AND** a refused check SHALL change no item
- **AND** if a conversion or the activation fails, AIGW SHALL return the
  converted items to the active program and leave that program in place
- **AND** a retained program that reads AIGW-owned items SHALL need no
  conversion.

#### Scenario: A package manager downgrades AIGW

- **WHEN** `aigw self rollback` runs on a Homebrew installation
- **THEN** AIGW SHALL convert each configured provider's items back without
  changing Homebrew-owned files and direct the operator to Homebrew for the
  earlier release
- **AND** it SHALL NOT refuse because a client declares retry budgets
- **AND** if a conversion fails, AIGW SHALL return the converted items and report
  that the installed program is unchanged
- **AND** if the configuration that names those providers cannot be read or
  decoded, AIGW SHALL change no item and SHALL NOT report the credentials ready
- **AND** if the operator keeps the release, its next Token use SHALL migrate the
  items again
- **AND** without that preparation AIGW SHALL NOT claim credential continuity,
  and the predecessor's reader MAY ask macOS to authorize each migrated item.

### Requirement: Recovery advice follows the selected credential backend

When AIGW recommends providing or replacing a provider's Token or balance
credential, the recommendation SHALL be one that the selected credential
backend can perform. A writable backend SHALL name the AIGW command that stores
the credential. The read-only environment backend SHALL name the exact
environment variables to set and SHALL NOT recommend a command that it refuses.

#### Scenario: The endpoint rejects an environment Token

- **WHEN** `aigw check` receives HTTP 401 for a provider whose Token comes from
  the environment backend
- **THEN** its human and JSON output SHALL name that provider's Token variable
- **AND** SHALL NOT recommend `aigw provider rotate`.

#### Scenario: Environment diagnostic credentials are absent

- **WHEN** `aigw provider balance` finds no balance credential for a provider in
  the environment backend
- **THEN** it SHALL name both balance credential variables for that provider
- **AND** SHALL NOT recommend `aigw provider edit --balance-token-stdin`.

#### Scenario: A writable backend needs a credential

- **WHEN** the same failures occur with a writable backend
- **THEN** the recommendation SHALL be `aigw provider rotate <provider>`, or
  `aigw provider edit <provider>` with `--balance-token-stdin` and
  `--balance-user-id`.

## REMOVED Requirements

### Requirement: Configuration evolution preserves retained credential readers

**Reason**: The schema 8 `aigw.toml` replaces `config.toml` and its component
files, and the credential reader decodes only a frozen view, so no capability
needs a separate component file to keep readers working.

**Migration**: "The credential reader decodes a frozen view" in this capability
keeps running readers working as fields are added, and "Upgrading imports a
schema 6 configuration once" owns the old files until they are retired.
