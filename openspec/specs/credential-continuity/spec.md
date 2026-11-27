# credential-continuity Specification

## Purpose

Define how provider Tokens stay readable and owned across upgrade, import,
rollback and configuration evolution.

## Requirements

### Requirement: Upgrade recovery preserves credential ownership

Candidate, rollback, and re-upgrade SHALL preserve configuration and credential
ownership without ACL change, backend migration, fallback, helper substitution
or a change of an item's owner. A rollback SHALL move no credential, because a
signed release reads the items that another signed release stored. Complete
rollback SHALL restore a compatible program and configuration.

#### Scenario: Rollback follows a failed candidate

- **WHEN** a candidate fails after replacing the installed program
- **THEN** rollback SHALL restore a program that reads the retained item as it
  is
- **AND** the original client invocation SHALL work without fallback, ACL change,
  or a new helper.

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

### Requirement: The credential reader decodes a frozen view

The credential reader SHALL decode from `aigw.toml` only each provider's ID and endpoints and each client's enabled
state, so that an enabled client is served by its provider's Token. It SHALL ignore `version`, every other field and
unknown protocol keys, match fingerprints exactly and check no client's selection. The file SHALL keep its name across
schema 9, because every reader since 0.7.0 reads a client without `authentication` as one that its provider's Token
serves.

#### Scenario: Unknown fields and a newer version are ignored

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

#### Scenario: An unknown protocol key is ignored

- **WHEN** a provider's endpoints hold a protocol key this reader does not know
- **THEN** the reader SHALL ignore that key
- **AND** it SHALL still return the Token for a fingerprint that matches an
  endpoint of a protocol it knows.

#### Scenario: The reader refuses a client it cannot serve

- **WHEN** the named client is disabled, or no provider endpoint matches the fingerprint
- **THEN** the reader SHALL return no Token and SHALL read no credential.

#### Scenario: The reader checks no selection

- **GIVEN** a running session's credential command carries the fingerprint of
  a provider endpoint that its client no longer selects
- **WHEN** the session calls the reader while that provider and endpoint exist
- **THEN** the reader SHALL return that provider's Token
- **AND** once the provider or that endpoint is gone, it SHALL return no Token.

#### Scenario: A client section still holds authentication

- **GIVEN** an enabled client's section in `aigw.toml` still holds `authentication`, as a version 8 file can
- **WHEN** that client calls the reader with a fingerprint that matches one of its providers' endpoints
- **THEN** the reader SHALL return that provider's Token, whatever `authentication` holds.

#### Scenario: A running session keeps its Token across schema 9

- **GIVEN** a running session's credential command names the reader of a release from 0.7.0 to 0.11.0
- **WHEN** the operator edits `aigw.toml` to schema 9, keeping that provider and endpoint, and `aigw sync` runs
- **THEN** that command SHALL keep returning its provider's Token without a client reload.

### Requirement: AIGW reads and writes only aigw.toml

AIGW SHALL take its configuration only from `aigw.toml` and SHALL NOT read,
write, rename or delete `config.toml` or its component files, so that a
credential reader of an earlier release keeps reading them. A home that holds
only those files has no configuration, and Token items SHALL stay unchanged.

#### Scenario: A home holds only the files of an earlier release

- **GIVEN** `config.toml` and its component files exist and `aigw.toml` does
  not
- **WHEN** an `aigw` command runs
- **THEN** it SHALL behave as in a home without configuration, naming
  `aigw setup` where it needs one
- **AND** `config.toml`, its component files and every Token item SHALL keep
  their bytes.

#### Scenario: Setup runs beside the files of an earlier release

- **GIVEN** `config.toml` exists and `aigw.toml` does not
- **WHEN** `aigw setup` writes `aigw.toml` and client configuration files
- **THEN** `config.toml`, its component files and every existing versioned
  credential reader SHALL keep their bytes
- **AND** a credential command that a running session loaded from an earlier
  release SHALL keep returning its Token from them.

### Requirement: A Keychain item of another program is refused with its fix

On macOS, AIGW SHALL use a Keychain item only through its own signing identity
and SHALL NOT run another program to read, replace or delete one. When macOS
refuses AIGW the read of an item while its keychain is unlocked, AIGW SHALL NOT
replace the item, and the failing command SHALL say that another program
stored it and SHALL name one fix: have the credential at hand, delete the item
with the system tool, then store it again. No command SHALL change an item's
owner.

#### Scenario: A release meets an item that 0.3.5 or earlier stored

- **WHEN** a command reads or replaces a slot whose item trusts only
  `/usr/bin/security`
- **THEN** it SHALL fail without a Token, without a prompt and without running
  that tool
- **AND** the item, its value, its access list and every other item SHALL be
  unchanged, although macOS would accept a new value for an item that it does
  not let AIGW read
- **AND** the failure SHALL name
  `security delete-generic-password -s AIGW_TOKEN -a <slot>` and a last step
  that stores the credential again: for a Token
  `aigw provider rotate <provider>`, unless a scenario below names another.

#### Scenario: The configuration does not hold the provider yet

- **WHEN** `aigw provider add` or `aigw setup` meets such an item for a provider
  that the command names and the configuration does not hold
- **THEN** the last step of its fix SHALL be that command's own line, or the
  same command in prose when a word would need quoting, because that command
  stores the Token
- **AND** `aigw setup --from` SHALL add `--token-stdin` to that line when the
  command had no Token input and could not ask for one
- **AND** for a provider that the configuration holds, the last step SHALL be
  the credential's own command
- **AND** a preview of `aigw setup --from`, which saves nothing, SHALL name as
  the last step for a waiting provider that the configuration does not hold
  that setup without `--dry-run`, naming the provider
- **AND** guided setup SHALL stop at the refusal before it asks for a model.

#### Scenario: A command reconciles several clients

- **WHEN** `aigw sync`, `aigw config rollback`, or `aigw setup --from` for a
  provider that it does not name, would write the files of a client whose
  provider's Token slot holds such an item
- **THEN** it SHALL complete for the other clients and SHALL NOT write that
  client's files, as for a client whose Token is missing
- **AND** it SHALL name the item's fix as the next step: `aigw setup --from` in
  that client's row, and `aigw sync` and `aigw config rollback` as their one
  next step, for the first client that waits
- **AND** its preview SHALL plan the same clients as its apply.

#### Scenario: A printed fix is followed

- **WHEN** the item that a printed fix names is deleted and its further steps
  are run as printed
- **THEN** each step SHALL succeed and the credential SHALL be stored
- **AND** the command that met the refusal SHALL no longer meet it
- **AND** a client that waited for that Token and whose files differ SHALL get
  them from `aigw sync`, which `aigw check --depth local` names until then.

#### Scenario: A command reads the credential for another purpose

- **WHEN** `aigw model search`, `aigw model add` or `aigw provider balance`
  reads such an item, or `aigw use`, `aigw client enable` or `aigw client edit`
  with configuration files selects, enables or moves the client that needs it
- **THEN** it SHALL end with the title and the fix of the refusal, and
  `aigw use`, `aigw client enable` and `aigw client edit` SHALL change nothing
- **AND** its JSON result SHALL name no single next command.

#### Scenario: A command observes whether a Token is stored

- **WHEN** `aigw check --depth local`, `aigw status` or `aigw setup` observes a
  slot that holds such an item
- **THEN** it SHALL report the refusal with its fix
- **AND** it SHALL NOT count the item as a stored Token
- **AND** a next step that offers that provider to a client that is not
  enabled SHALL name that fix for it, not the credential's command alone
- **AND** `aigw setup --from` SHALL end with the refusal only for a provider
  that the command names; any other provider SHALL wait with the fix while the
  setup completes for the rest.

#### Scenario: A stored Token is empty

- **WHEN** a command would write the files of a client whose provider's Token
  slot is present and holds no value
- **THEN** it SHALL treat that client as one whose Token is missing, on every
  credential backend
- **AND** it SHALL NOT fail as an unverified credential reader.

#### Scenario: A client file lists the providers that the client can use

- **WHEN** AIGW writes a client file that lists every provider with a Token
- **THEN** it SHALL list no provider whose Token slot is empty or holds such an
  item, as it lists none without a Token.

#### Scenario: The provider of such an item is removed

- **WHEN** `aigw provider remove` deletes a slot whose item another program
  stored
- **THEN** the item SHALL be removed without a prompt and without running that
  tool, as an item of AIGW's own is
- **AND** every other item SHALL be unchanged.

#### Scenario: The keychain is locked

- **WHEN** macOS refuses a read or a replacement while the keychain that holds
  the item is locked, wherever it stands in the search list, or was locked when
  the operation began
- **THEN** the command SHALL fail without a prompt and without a second attempt
- **AND** its fix SHALL NOT tell the operator to delete the item.

#### Scenario: A later signed release reads the item

- **WHEN** a release with the same signing identifier and team reads an item
  that an earlier signed release stored
- **THEN** it SHALL return the Token without a prompt and without changing the
  item.

#### Scenario: A program rollback runs

- **WHEN** `aigw self rollback` activates the retained program
- **THEN** no Keychain item SHALL change
- **AND** the retained program SHALL read the items as they are.

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
