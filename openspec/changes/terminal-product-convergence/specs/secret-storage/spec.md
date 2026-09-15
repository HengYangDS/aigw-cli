## ADDED Requirements

### Requirement: Explicit Token input is complete and scoped

Every `--token-stdin` operation SHALL read through EOF within a 64 KiB input
budget and admit one non-empty visible ASCII Token with at most one terminal LF
or CRLF. It SHALL reject malformed or incomplete input without credential,
configuration or network effects. It SHALL NOT silently trim whitespace,
discard extra lines or consult ambient credentials instead.

Endpoint `test --token-stdin` SHALL require one explicit Profile or client,
resolve its Account and endpoint before consuming input, and use the Token only
for that request. It SHALL neither read nor write the credential store or client
state. Its optional `--config` SHALL require an absolute configuration path and
SHALL NOT change default path selection for another operation. The result SHALL
remain HTTP endpoint observation, not model inference or client verification.

Raw stdin SHALL remain literal by default. An explicitly selected
`--token-format go-keyring-base64` SHALL decode exactly one canonical storage
envelope produced by the pinned macOS backend and validate the decoded Token
without trimming it. Unknown formats, missing or malformed envelopes, nested
encoding and invalid decoded Tokens SHALL fail before HTTP. The option SHALL
require `--token-stdin` and SHALL NOT alter the credential store.

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

- **WHEN** the operator selects one Account-Token Profile and supplies a valid
  Token with `--token-stdin` and an absolute configuration file
- **THEN** only that Profile's endpoint receives the Token
- **AND** configuration, credential stores and client files remain unchanged.

#### Scenario: Ephemeral input lacks an unambiguous credential owner

- **WHEN** an ephemeral endpoint test omits the target, selects an unknown
  Profile or selects client-owned authentication
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

AIGW SHALL expose the selected credential backend, its observed availability,
declared mutability, persistence and recovery action without retrieving secret
values. `read_write` describes the backend interface, not proof that an
operating-system credential operation will be authorized or complete. Metadata
observation SHALL NOT certify future secret access or durable writes.
Automatic selection SHALL be deterministic for the installation and SHALL NOT
silently cross-read another backend. Read-only observation and credential reads
SHALL NOT persist a previously unrecorded automatic selection; the first
credential mutation SHALL persist it before changing credential state.
Every later ordinary mutation SHALL validate the current persisted selection
against its resolved backend before writing or deleting a credential.
Inspection SHALL derive persistence from current selection metadata rather than
an earlier in-memory observation. Conflicting metadata SHALL NOT silently change
the invocation's resolved backend.

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

An Account Token SHALL be required only when an explicit operation activates,
projects, checks, or verifies a Route with Account-Token authentication.
Client-native authentication SHALL remain independent of AIGW Token slots.
Readiness checks SHALL admit the selected client's executable and configuration
projection before reading its Token value or authenticating its endpoint.
An unready projection SHALL retain its own recovery action in both human and
JSON output; an unobserved credential SHALL NOT be classified as absent.

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

## MODIFIED Requirements

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
