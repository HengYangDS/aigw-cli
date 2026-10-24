# secret-storage Specification

## Purpose

Define secure, deterministic Account Token storage that remains usable across
supported operating systems without making a native credential service a
continuous runtime prerequisite.

## Requirements

### Requirement: Native credential service failure has a portable outcome

On supported platforms, automatic resolution SHALL honor an existing backend
choice. Only an unrecorded choice MAY probe native metadata and select the
platform-safe file fallback on failure. The probe SHALL not read Tokens or
claim future read/write permission. Unix fallback SHALL use owner-only storage;
Windows fallback SHALL protect values with current-user DPAPI.

#### Scenario: Headless Linux has no usable credential service

- **WHEN** no backend is recorded and native metadata probing fails on Linux
- **THEN** AIGW selects the secure file backend
- **AND** setup can persist one supplied provider Token

#### Scenario: macOS credential service is unavailable

- **WHEN** no backend is recorded and native metadata probing fails on macOS
- **THEN** AIGW selects the secure file backend without prompting through a client application

#### Scenario: Explicit keyring selection fails closed

- **WHEN** the operator explicitly selects the native credential service
- **AND** that service is unavailable
- **THEN** AIGW reports the backend failure
- **AND** does not silently select the file backend

#### Scenario: Windows automatic selection

- **WHEN** Windows resolves an unrecorded automatic backend
- **THEN** successful native metadata probing SHALL select Credential Manager
- **AND** a failed probe SHALL select the current-user DPAPI file backend
- **AND** only a credential mutation SHALL persist that automatic choice

### Requirement: File persistence is owner-only and atomic

The file backend SHALL use a verified owning directory and regular Token
files, with guarded same-directory replacement. Unix SHALL enforce current-user
ownership, owner-only modes and link-count restrictions. Windows SHALL use
current-user DPAPI with interaction forbidden; Unix mode bits SHALL NOT be
presented as the Windows protection boundary.

#### Scenario: Fresh file store

- **WHEN** AIGW first persists a Token in the file backend
- **THEN** Unix SHALL require directory mode `0700` and Token file mode `0600`
- **AND** Windows SHALL persist only DPAPI-protected bytes bound to the current
  user, without claiming Unix permission semantics

#### Scenario: Unsafe storage object

- **WHEN** the selected platform detects an invalid root, a nonregular Token
  file or a path-identity change
- **THEN** AIGW SHALL fail before returning or changing Token material
- **AND** Unix SHALL additionally reject symbolic or multiply linked files,
  unsafe modes and foreign ownership

### Requirement: One usable provider is sufficient

Setup SHALL require Tokens only for Accounts selected by the requested Routes;
unselected catalogue Accounts SHALL remain optional.

#### Scenario: Team manifest has one available provider

- **WHEN** a team manifest declares several Accounts
- **AND** the selected Routes require only one Account with an available Token
- **THEN** setup completes without requesting Tokens for unselected Accounts

### Requirement: One backend owns credential persistence

AIGW SHALL select exactly one credential backend for an invocation. The same
backend SHALL own API Tokens and provider-diagnostic credentials in distinct
typed slots, and AIGW SHALL NOT read or write another backend as a
compatibility fallback.

#### Scenario: Automatic selection remains stable

- **WHEN** an unrecorded automatic backend receives its first credential mutation
- **THEN** AIGW SHALL persist that selection before changing the credential
- **AND** subsequent invocations SHALL reuse it for every credential purpose
- **AND** read-only observation or retrieval SHALL not create selection state

#### Scenario: Credential purposes remain isolated

- **WHEN** one Account has an API Token and a provider-diagnostic credential
- **THEN** each value is read, replaced, and deleted through its own typed slot
- **AND** an operation on one purpose SHALL NOT change the other

#### Scenario: Environment storage is explicit and read-only

- **WHEN** the operator selects the environment backend
- **THEN** AIGW reads only documented Account credential environment variables
- **AND** refuses credential writes and deletes

#### Scenario: Environment Account names cannot collide

- **WHEN** two valid Account IDs differ by dot, dash, or underscore
- **THEN** AIGW derives distinct deterministic environment variable names
- **AND** the mapping identifies the original lowercase Account ID without an
  ambiguous normalization rule

#### Scenario: Diagnostic environment credential is incomplete

- **WHEN** only the diagnostic system token or only the diagnostic user ID is
  present for an Account
- **THEN** AIGW treats the provider-diagnostic credential as unavailable
- **AND** never substitutes the Account API Token or another backend

### Requirement: Credential availability is observable without disclosure

AIGW SHALL observe whether an Account credential is present without retrieving
its secret value. The observation SHALL distinguish present, absent, and
backend failure; it SHALL NOT report a backend failure as absence.

#### Scenario: Read-only journey observes a present credential

- **WHEN** status, setup, sync, route, adapter, manifest, doctor, or
  catalogue logic needs only credential availability
- **THEN** AIGW queries credential metadata without retrieving the value
- **AND** does not initiate authentication or credential mutation

#### Scenario: Credential backend observation fails

- **WHEN** the selected credential backend cannot determine availability
- **THEN** AIGW reports the backend failure
- **AND** does not describe the credential as missing

#### Scenario: Authentication needs the credential value

- **WHEN** AIGW performs an explicit provider or client authentication action
- **THEN** it retrieves the credential value from the same selected backend
- **AND** no availability cache or alternate backend becomes authoritative

### Requirement: Native availability observation is non-interactive

AIGW SHALL use value-free native metadata operations to observe credentials on
macOS, Linux, and Windows. Observation SHALL NOT request secret disclosure or
open a credential-access prompt.

#### Scenario: macOS Keychain observation

- **WHEN** AIGW observes a generic-password item on macOS
- **THEN** it queries item metadata without requesting password data

#### Scenario: Linux Secret Service observation

- **WHEN** AIGW observes a Secret Service item on Linux
- **THEN** it searches item attributes without opening a secret session or
  requesting secret bytes

#### Scenario: Linux Secret Service is unavailable

- **WHEN** an explicitly selected or already persisted native backend cannot
  connect to Secret Service while observing a credential on Linux
- **THEN** it reports the connection failure
- **AND** it does not read credential values, select another backend, or open an
  interactive prompt

#### Scenario: Windows Credential Manager observation

- **WHEN** AIGW observes a generic credential on Windows
- **THEN** it uses credential metadata to determine presence without exposing
  the credential blob to AIGW

### Requirement: Explicit Token input is complete and scoped

Every `--token-stdin` operation SHALL read one complete Token through EOF within
64 KiB, preserving visible ASCII except one terminal LF or CRLF.
Malformed, incomplete, multiline, or ambiguous input SHALL fail before
credential, configuration, or network effects. Endpoint testing SHALL resolve
one Route or client before consuming input and use it only for that request.
Explicit storage-envelope decoding SHALL validate one canonical format without
reading or changing the credential store.

#### Scenario: Complete Token arrives in multiple pipe writes

- **WHEN** the Token arrives in fragments and the producer later closes stdin
- **THEN** the operation uses the complete Token only after EOF
- **AND** a read failure after a first line fails rather than admitting that line.

#### Scenario: Input contains an extra line or injected header

- **WHEN** input contains embedded CR, LF, NUL, whitespace, non-ASCII characters,
  more than one final line ending, or exceeds the byte budget
- **THEN** the operation rejects it before using the credential or making HTTP
  requests and does not expose the input in its diagnostic.

#### Scenario: Endpoint test consumes an explicitly supplied Token

- **WHEN** the operator selects one Account-Token Route and supplies a valid
  Token with `--token-stdin` and an absolute configuration file
- **THEN** only that Route's endpoint receives the Token
- **AND** configuration, credential stores and client files remain unchanged.

#### Scenario: Ephemeral input lacks an unambiguous credential owner

- **WHEN** an ephemeral endpoint test omits the target, selects an unknown
  Route or selects client-owned authentication
- **THEN** it fails before stdin consumption, credential access or HTTP.

#### Scenario: A native broker returns macOS storage bytes

- **WHEN** the endpoint test explicitly selects the go-keyring-base64 format
- **THEN** the exact canonical envelope is decoded once and only the validated
  decoded Token reaches the selected endpoint
- **AND** raw mode never implicitly decodes a prefix-looking Token.

### Requirement: Authenticated probes stay at the selected endpoint

Credential validation, endpoint tests, readiness and provider-account diagnostics
SHALL share one request execution boundary that does not follow HTTP redirects or mutate the
caller's native HTTP client. A redirect SHALL NOT establish successful
authentication or a healthy diagnostic result.

#### Scenario: An authenticated endpoint redirects to another origin

- **WHEN** the selected endpoint returns an HTTP redirect
- **THEN** the probe returns that response without contacting its destination
- **AND** the result reports the redirect rather than success at another origin.

#### Scenario: Provider-account JSON observation is incomplete

- **WHEN** a provider-account response exceeds its byte budget, contains extra
  JSON content, or fails while reading or closing
- **THEN** the diagnostic fails without reporting a successful account result
- **AND** original read and close failures remain inspectable.

#### Scenario: Provider-account pagination exhausts its budget

- **WHEN** every permitted search page remains full
- **THEN** the diagnostic reports an incomplete search
- **AND** does not claim the selected Token is absent.

### Requirement: Credential backend state is explicit and portable

AIGW SHALL expose backend selection, availability, mutability, persistence, and
recovery without reading secrets. Interface capability SHALL NOT prove host
authorization. Automatic selection SHALL be deterministic and never cross-read
a backend. Observation and reads SHALL NOT persist a new selection; the first
mutation SHALL persist it before credential effects, and later mutations SHALL
revalidate it. Inspection SHALL use current metadata; conflicts SHALL NOT
silently switch backend.

#### Scenario: Native credential storage is usable

- **WHEN** an unrecorded automatic selection admits a native backend through
  non-interactive metadata observation and a credential mutation is requested
- **THEN** automatic selection records that native backend before changing the
  credential
- **AND** later commands reuse it without opening an access prompt merely to
  test presence.

#### Scenario: Native credential storage is unavailable

- **WHEN** automatic selection proves the native service unavailable during a
  credential mutation
- **THEN** AIGW records the declared platform-safe fallback before changing the
  credential, or reports that no writable backend is available
- **AND** the result names the exact recovery action without repeated prompts.

#### Scenario: Read-only access resolves an unrecorded backend

- **WHEN** a read-only command or credential read resolves an automatic backend
  that has not yet been persisted
- **THEN** it may use that backend for the current invocation
- **AND** the backend-selection state remains absent.

#### Scenario: Environment credentials are selected

- **WHEN** the operator explicitly selects environment-backed credentials
- **THEN** setup, sync, check, and client helpers read the documented variables
  consistently
- **AND** every credential mutation reports that the backend is read-only.

#### Scenario: Selection metadata changes after inspection

- **WHEN** an invocation inspects again after another writer adds, removes or
  changes its persisted backend choice
- **THEN** a matching choice is reported as persisted and an absent choice as
  deferred; a different or invalid choice produces an unavailable result
- **AND** inspection preserves that metadata, does not probe another backend
  and does not read credential values.

#### Scenario: The backend choice changes after an earlier mutation

- **WHEN** an invocation has already written a credential and the persisted
  backend choice subsequently names a different or invalid backend
- **THEN** its next credential mutation stops before touching the credential
- **AND** preserves both the credential and the externally changed choice.

#### Scenario: The persisted choice is removed between mutations

- **WHEN** a later credential mutation finds the persisted backend choice absent
- **THEN** it persists the resolved backend again before changing the credential.

### Requirement: Credential availability is scoped to active demand

An Account Token SHALL be required only for an explicit operation on an active
Account-Token Route. Client-native authentication SHALL remain independent.
Readiness SHALL admit the selected executable and projection before reading a
Token or probing an endpoint. An unready projection SHALL retain its human and
JSON recovery action; an unobserved credential SHALL NOT be classified as absent.

#### Scenario: A selected client's projection is not ready

- **WHEN** `aigw check` observes a missing executable, missing target or invalid
  projection for an enabled client
- **THEN** it reports that client's projection problem and recovery action
- **AND** it reads no Token value and makes no authenticated request for that
  client, even if its Token is missing or unreadable.

#### Scenario: A selected client's projection is ready

- **WHEN** the enabled client's projection passes local checks
- **THEN** an Account-authenticated Route reads its Token once before endpoint
  diagnostics, retaining credential failure details when the read fails
- **AND** client-owned authentication reads no AIGW credential or endpoint.

#### Scenario: Catalogue contains unused Accounts

- **WHEN** a reviewed manifest contains Accounts not selected by enabled Routes
- **THEN** their missing Tokens do not block setup, synchronization, status, or
  readiness for the active Routes.

### Requirement: Credential compensation is postimage-guarded

A failed credential mutation that became externally visible SHALL restore the
exact preimage only while compensation observes AIGW's own postimage. If
compensation observes another writer's state, AIGW SHALL preserve it and report
that compensation was not applied. Successful cleanup SHALL leave no owned
temporary file or partial Token; cleanup failure SHALL retain the exact owned
resource and original failure without claiming restoration.

#### Scenario: Durable replacement or deletion fails

- **WHEN** AIGW changes a credential slot but cannot prove the change durable
- **THEN** it applies guarded restoration of the exact preimage and cleanup of
  files owned by that attempt
- **AND** successful recovery restores the preimage and leaves no owned staging
  residue; failure reports the unresolved resource and every retained cause.

#### Scenario: The credential changes before compensation

- **WHEN** the current slot no longer equals AIGW's own postimage
- **THEN** compensation refuses to overwrite or delete the newer state
- **AND** the error identifies both the failed operation and the incomplete
  compensation.

#### Scenario: The backend choice changes before compensation

- **WHEN** a replacement must compensate after another writer changes the backend
  choice
- **THEN** compensation restores its unchanged credential in the original backend
  and credential kind
- **AND** preserves the newer backend choice and reports its compensation conflict.

### Requirement: Native credential qualification owns real store access

Ordinary source tests SHALL use provider doubles and avoid host credential
stores. A retained-item journey SHALL run only on an explicitly admitted
disposable host, preserve the original item and reader identity, and remain
independent of publisher signing. The scope statement SHALL NOT claim the host
is isolated or disposable. Source coverage or a fresh helper SHALL NOT qualify
retained-item, existing-caller, real-client, or distribution continuity.

#### Scenario: A review runner has no publisher signing identity

- **GIVEN** an explicitly admitted disposable macOS runner
- **WHEN** ordinary native acceptance executes
- **THEN** source tests use provider doubles and do not touch the host Keychain
- **AND** the explicit system-store journey may run without publisher signing
  inputs
- **AND** distribution trust remains a separate release claim.

#### Scenario: Ordinary source verification isolates native credentials

- **WHEN** source verification exercises a native credential worker
- **THEN** it SHALL use the injected provider boundary without accessing the
  operator credential store
- **AND** this result SHALL NOT qualify retained system credentials.

#### Scenario: Explicit Keychain qualification has no disposable-host scope

- **WHEN** macOS native qualification enables system credentials without
  `AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE=ephemeral-host`
- **THEN** it SHALL fail before executing any quality, test, or release command
- **AND** directly selected integration tests SHALL reject that scope before
  native credential access.

#### Scenario: Explicit Keychain qualification uses a disposable host

- **WHEN** macOS native qualification selects both system credentials and the
  disposable-host scope
- **THEN** the existing coverage invocation SHALL include integration tests once
- **AND** selecting full quality SHALL preserve that same inclusion and scope.

#### Scenario: A retained macOS item keeps its reader identity

- **WHEN** a candidate replaces a predecessor that reads an existing Keychain item
- **THEN** predecessor, candidate, and rollback SHALL keep the item's physical
  service and Account identity
- **AND** a candidate SHALL reach a `/usr/bin/security` item only through the
  installed program's one-time migration to AIGW ownership or a versioned
  reader's read through that tool
- **AND** a fresh or replacement helper SHALL NOT establish retained-item access.

### Requirement: Explicit host credential policy survives client projection

A local Adapter MAY select one trusted absolute `credential_command` using the
existing client-and-fingerprint contract. It SHALL change neither AIGW's native
credential implementation nor backend authority. Sync, upgrade, and explicit
reenablement SHALL preserve it; disable SHALL retain dormant policy and uninstall
SHALL remove only Adapter policy. Verification SHALL use the synchronized helper
through the bounded client runner. Readiness SHALL claim projection state only.

#### Scenario: A trusted external reader supplies the client credential

- **GIVEN** an enabled Adapter with an explicit absolute credential executable
- **WHEN** sync, program upgrade and live client verification execute
- **THEN** sync and upgrade preserve the helper and do not read a Token
- **AND** verification uses the synchronized native client, not a fallback reader
- **AND** disabled policy remains disabled until explicit re-enablement
- **AND** an older binary that cannot parse this policy is rejected before rollback.

### Requirement: Native credential operations are bounded

AIGW SHALL run each native credential operation in a private subprocess with a
five-second deadline and bounded cleanup, passing only the platform identity its
store requires. A timeout or authorization failure SHALL end the exact operation
without retry, fallback, access-policy change, or backend migration, apart from
the macOS retained-item handling below. Creation or timing SHALL NOT prove later
authorization.

#### Scenario: The installed executable creates and rotates a credential

- **WHEN** its native writer creates or updates an authorized exact slot
- **THEN** its native reader returns the written value without interaction
- **AND** mutation emits no Token, including unexpected backend output.

#### Scenario: Exact deletion is repeated

- **WHEN** an authorized deletion removes an item and deletion is requested again
- **THEN** both operations succeed and the exact item remains absent.

#### Scenario: Credential presence is observed

- **WHEN** AIGW checks an exact native credential slot
- **THEN** the subprocess returns only present or absent metadata
- **AND** no credential value enters the parent process.

#### Scenario: A credential operation fails or requires authorization

- **WHEN** the exact native item cannot be accessed within the admitted operation
  and is neither a retained item that the installed program migrates nor one
  that another AIGW program reads through `/usr/bin/security`
- **THEN** the subprocess fails without retrying or returning a Token
- **AND** no credential or access-control state changes
- **AND** any operating-system authorization UI remains an explicit platform
  behavior rather than a suppressed or disproved event.

#### Scenario: A concurrent migration completes between lookups

- **WHEN** a macOS worker is denied or misses the exact slot and then finds
  neither a retained nor a staged item for it
- **THEN** it SHALL read the slot once more without UI, because another worker
  may have just recreated it as an AIGW-owned item
- **AND** SHALL return that read's result without a further attempt.

#### Scenario: The native service does not answer

- **WHEN** the operation reaches its deadline
- **THEN** the parent terminates and reaps its subprocess through bounded cleanup
- **AND** the helper reports the deadline rather than suggesting configuration sync.

### Requirement: Native credential transport preserves secret boundaries

Tokens SHALL enter the native credential subprocess only through bounded
standard input. Metadata operations SHALL return presence only. Diagnostic
output SHALL never share Token standard output. Encoding SHALL occur once, and
updates SHALL preserve item identity and policy. Every failure SHALL return no
secret and leave credential and access-control state unchanged.

#### Scenario: Native credential output contains diagnostics

- **WHEN** a native provider reports diagnostic output during a credential write
- **THEN** AIGW SHALL keep the diagnostic separate from Token output
- **AND** the diagnostic SHALL NOT be interpreted as credential data.

#### Scenario: A retained released item is consumed after replacement

- **WHEN** the candidate replaces a published predecessor using the same provider
- **THEN** each original client credential command is executed before sync
- **AND** update, rollback and re-upgrade return the retained Token without
  backend migration, helper replacement or configuration drift.

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
