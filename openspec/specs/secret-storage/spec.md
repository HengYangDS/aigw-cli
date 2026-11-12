# secret-storage Specification

## Purpose

Define secure, deterministic storage of provider Tokens and balance credentials
that remains usable across supported operating systems without making a native
credential service a continuous runtime prerequisite.

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

### Requirement: Credential availability is observable without disclosure

AIGW SHALL observe whether a provider's Token or balance credential is present
without retrieving its secret value. The observation SHALL distinguish present,
absent, and backend failure; it SHALL NOT report a backend failure as absence.

#### Scenario: Read-only journey observes a present credential

- **WHEN** status, setup, sync, local checks, provider and model listings, or
  client configuration logic needs only credential availability
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

Input that `--token-stdin` or `--balance-token-stdin` reads without a terminal
SHALL be one complete value through EOF within 64 KiB, preserving visible ASCII
except one terminal LF or CRLF. Malformed, incomplete, multiline, or ambiguous
input SHALL fail before credential, configuration, or network effects. Each
operation SHALL resolve the one provider that receives the value before reading
it and use it as given; a one-time Token SHALL never be stored.

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

- **WHEN** `aigw check` receives a one-time Token through `--token-stdin`, as
  "Checking a named model leaves every selection unchanged" in
  `model-selection` admits it
- **THEN** AIGW SHALL use that Token only for that check
- **AND** it SHALL neither read the stored Token nor store the one-time Token.

#### Scenario: Ephemeral input lacks an unambiguous credential owner

- **WHEN** a `--token-stdin` operation does not resolve exactly one provider
  whose Token AIGW would send, such as a check of a client that uses its own
  authentication
- **THEN** it fails before stdin consumption, credential access or HTTP.

#### Scenario: A native broker returns macOS storage bytes

- **WHEN** standard input carries a Token in the go-keyring-base64 storage
  envelope that a native broker returned
- **THEN** AIGW uses those bytes as the raw Token and never decodes them, so a
  broker that reads the native store decodes its envelope itself
- **AND** no flag selects another input encoding.

### Requirement: Authenticated probes stay at the selected endpoint

Token validation, `aigw check` requests and balance probes SHALL share one
request execution boundary that does not follow HTTP redirects or mutate the
caller's native HTTP client. A redirect SHALL NOT establish successful
authentication or a healthy balance result.

#### Scenario: An authenticated endpoint redirects to another origin

- **WHEN** the selected endpoint returns an HTTP redirect
- **THEN** the probe returns that response without contacting its destination
- **AND** the result reports the redirect rather than success at another origin.

#### Scenario: Provider-account JSON observation is incomplete

- **WHEN** a balance response exceeds its byte budget, contains extra JSON
  content, or fails while reading or closing
- **THEN** the balance probe fails without reporting a balance
- **AND** original read and close failures remain inspectable.

#### Scenario: Provider-account pagination exhausts its budget

- **WHEN** every permitted search page remains full
- **THEN** the balance probe reports an incomplete search
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
  service and account identity
- **AND** a candidate SHALL reach a `/usr/bin/security` item only through the
  installed program's one-time migration to AIGW ownership or a versioned
  reader's read through that tool
- **AND** a fresh or replacement helper SHALL NOT establish retained-item access.

### Requirement: Explicit host credential policy survives client file writes

A client MAY select one trusted absolute `credential_command` using the existing
client-and-fingerprint contract. It SHALL change neither AIGW's native
credential implementation nor backend authority. Sync, update and
re-enablement SHALL preserve it; disable SHALL keep it dormant and uninstall
SHALL remove only its client policy. `aigw check` SHALL prove it only through a
bounded real client session.

#### Scenario: A trusted external reader supplies the client credential

- **GIVEN** an enabled client with an explicit absolute credential command
- **WHEN** `aigw sync`, a program update and `aigw check --depth client` run
- **THEN** sync and the update preserve the command and do not read a Token
- **AND** the `client` depth uses the synchronized native client, not a
  fallback reader, and the `endpoint` and `model` depths are skipped with
  their reason
- **AND** disabled policy remains disabled until `aigw client enable`
- **AND** an older program that cannot parse this policy is rejected before
  rollback.

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

#### Scenario: A credential subprocess exits at once

- **WHEN** a native credential subprocess, or a client credential command that
  AIGW runs, exits as soon as it starts or starts its own children at once
- **THEN** it SHALL already belong to the process group or Job that owns its
  cleanup before it runs any code, and its start SHALL succeed
- **AND** a subprocess that cannot be contained SHALL be stopped, with the
  cause reported.

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

### Requirement: Interactive Token input is never echoed

A prompt for a Token or balance token at an interactive terminal SHALL read it
with terminal echo disabled in every prompt mode, including the accessible mode
that `NO_COLOR` or `AIGW_ACCESSIBLE` selects, and SHALL NOT write it to the
prompt output. Such input that is not an interactive terminal SHALL be read
without being written to the prompt output.

#### Scenario: An accessible prompt reads a Token at a terminal

- **WHEN** `NO_COLOR` or `AIGW_ACCESSIBLE` selects accessible prompts and the
  operator types a Token at an interactive terminal
- **THEN** the terminal does not echo the Token
- **AND** the prompt output contains only the prompt label and a line ending.

#### Scenario: An accessible prompt reads a Token from a pipe

- **WHEN** accessible prompts read a Token from standard input that is not a
  terminal
- **THEN** the Token is read as one line
- **AND** the prompt output does not contain it.

#### Scenario: A balance token is prompted at a terminal

- **WHEN** `aigw provider edit <provider> --balance-user-id <id>` runs with
  `--balance-token-stdin` at an interactive terminal
- **THEN** it prompts for the balance token with terminal echo disabled
- **AND** the prompt output contains only the prompt label and a line ending.

### Requirement: A Token is stored only after its provider accepts it

Every command that stores a Token, including `aigw provider add`,
`aigw provider rotate`, `aigw setup` and `aigw use`, SHALL store it only after
one authenticated model-free request to that provider accepts it. When the
provider rejects the Token or the request cannot complete, the command SHALL
refuse and store nothing. Output SHALL NOT contain the Token.

#### Scenario: A provider is added with its Token

- **WHEN** `aigw provider add <provider> --endpoint <p>=<url> --token-stdin`
  reads a Token that the provider accepts
- **THEN** the Token SHALL be stored in that provider's Token slot of the
  selected backend
- **AND** human and JSON output SHALL report that it was validated and stored
  without containing it.

#### Scenario: The provider does not accept a new Token

- **WHEN** the provider rejects a Token that a command validates, or the
  request cannot complete
- **THEN** the command SHALL refuse and store nothing
- **AND** `aigw provider add` SHALL leave the provider unconfigured, and
  `aigw provider rotate` SHALL keep the previous Token.

#### Scenario: A Token is requested without a terminal

- **WHEN** `aigw provider add` or `aigw provider rotate` runs without a
  terminal, or with `--json`, and without `--token-stdin`
- **THEN** it SHALL refuse before reading input or sending a request, naming
  `--token-stdin`
- **AND** it SHALL write nothing.

#### Scenario: The environment backend cannot replace a Token

- **WHEN** the environment backend is selected and `aigw provider rotate`
  would replace a Token
- **THEN** the command SHALL refuse before reading input, naming the
  environment variable that supplies the Token
- **AND** no configuration or credential SHALL change.

### Requirement: Provider commands manage balance credentials

`aigw provider edit` SHALL store a provider's balance credential from
`--balance-token-stdin` together with `--balance-user-id`, and delete it with
`--no-balance-token`, without changing the provider's Token. Output SHALL NOT
contain it. `aigw provider remove` deletes it as "Providers are removed with
their models and Tokens" in `product-control-plane` specifies.

#### Scenario: Balance credentials are stored and removed

- **WHEN** `aigw provider edit <provider>` reads a balance token through
  `--balance-token-stdin` with `--balance-user-id <id>`
- **THEN** the balance credential SHALL be stored in its own slot and the
  provider's Token SHALL stay unchanged
- **AND** at a terminal, `--balance-token-stdin` SHALL prompt for the balance
  token as "Interactive Token input is never echoed" requires
- **AND** `--no-balance-token` SHALL delete the balance credential and leave
  the Token unchanged.

#### Scenario: The environment backend cannot store a balance credential

- **WHEN** the environment backend is selected and a balance flag of
  `aigw provider edit` would store or delete the balance credential
- **THEN** the command SHALL refuse before reading input, naming the
  environment variables that supply it
- **AND** no configuration or credential SHALL change.

### Requirement: One backend owns Tokens and balance credentials

AIGW SHALL select exactly one credential backend for an invocation. The same
backend SHALL own Tokens and balance credentials in distinct typed slots, and
AIGW SHALL NOT read or write another backend as a compatibility fallback.

#### Scenario: Automatic selection remains stable

- **WHEN** an unrecorded automatic backend receives its first credential mutation
- **THEN** AIGW SHALL persist that selection before changing the credential
- **AND** subsequent invocations SHALL reuse it for every credential purpose
- **AND** read-only observation or retrieval SHALL not create selection state

#### Scenario: Credential purposes remain isolated

- **WHEN** one provider has a Token and a balance credential
- **THEN** each value is read, replaced, and deleted through its own typed slot
- **AND** an operation on one purpose SHALL NOT change the other
- **AND** `aigw provider rotate` and the balance flags of `aigw provider edit`
  SHALL each change only their own slot

#### Scenario: Environment storage is explicit and read-only

- **WHEN** the operator selects the environment backend
- **THEN** AIGW reads only documented provider credential environment variables
- **AND** refuses credential writes and deletes

#### Scenario: Environment provider names cannot collide

- **WHEN** two valid provider IDs differ by dot, dash, or underscore
- **THEN** AIGW derives distinct deterministic environment variable names
- **AND** the mapping identifies the original lowercase provider ID without an
  ambiguous normalization rule

#### Scenario: The environment names the balance credential

- **WHEN** the environment backend reads a provider's balance credential
- **THEN** AIGW SHALL read it from `AIGW_BALANCE_TOKEN_<PROVIDER>` and
  `AIGW_BALANCE_USER_ID_<PROVIDER>`, and every fix SHALL name them
- **AND** AIGW SHALL read no variable name that a release before 0.7.0 read
  for it.

#### Scenario: A balance credential in the environment is incomplete

- **WHEN** only the balance token or only the balance user ID is present for a
  provider
- **THEN** AIGW treats the balance credential as unavailable
- **AND** never substitutes the provider's Token or another backend

### Requirement: Token availability is scoped to active demand

Readiness SHALL require a provider's Token only for models that clients use,
and client-native authentication SHALL remain independent. `aigw check` SHALL
pass a client's `local` depth before reading its Token or probing an endpoint,
and an unobserved Token SHALL NOT be reported as absent. `aigw model search`,
catalog checks and `aigw provider balance` SHALL read a selected provider's
Token when present.

#### Scenario: A client's configuration files are not ready

- **WHEN** `aigw check` finds a missing executable, missing target or invalid
  client configuration file for an enabled client
- **THEN** it reports that client's `local` failure and its fix
- **AND** it reads no Token value and makes no authenticated request for that
  client, even if its Token is missing or unreadable.

#### Scenario: A client's configuration files are ready

- **WHEN** the enabled client passes its `local` depth
- **THEN** a client that uses a provider Token reads it once before its
  `endpoint` depth, retaining credential failure details when the read fails
- **AND** client-owned authentication reads no AIGW credential and probes no
  endpoint.

#### Scenario: A team file contains unused providers

- **WHEN** a reviewed team file contains providers whose models no enabled
  client uses
- **THEN** their missing Tokens do not block setup, synchronization, status, or
  `aigw check` for the clients in use.

#### Scenario: A command reads a selected provider's Token

- **WHEN** `aigw model search`, `aigw provider balance` or a catalog check
  selects a provider, whether or not a client uses its models
- **THEN** it SHALL read that provider's Token when present
- **AND** a provider without a Token SHALL get its fix while the other selected
  providers still print, and the command SHALL exit 1, as for an unreadable
  catalog.

### Requirement: One connected provider is sufficient

Setup SHALL require a Token only for each provider it connects; other providers
in a team file SHALL remain optional.

#### Scenario: A team file has one available provider

- **WHEN** a team file declares several providers
- **AND** `aigw setup --from` connects only one of them, whose Token is in the
  selected backend or arrives through `--provider <id> --token-stdin`
- **THEN** setup completes without requesting Tokens for the other providers

### Requirement: A service's Tokens can be listed by name alone

AIGW SHALL list the provider IDs of a service's Tokens from item names or
attributes alone, in the same bounded private subprocess as other native
operations. The listing SHALL read no value, prompt for nothing and pass only
provider IDs across the subprocess boundary. An API that returns values with
the names, as Windows Credential Manager does, SHALL have them zeroed unread
inside the subprocess.

#### Scenario: A store needs a person to list

- **WHEN** a keychain or collection is locked
- **THEN** the listing SHALL name its items or refuse, and SHALL NOT ask for
  access
- **AND** a refusal SHALL be reported as unable to list, never as no Tokens
- **AND** no credential or access-control state SHALL change.
