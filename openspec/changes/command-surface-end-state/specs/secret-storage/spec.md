## ADDED Requirements

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

## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: One backend owns credential persistence

**Reason**: Its scenario names use the retired Account and diagnostic words,
and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "One backend owns Tokens and balance credentials" in
this capability, with every scenario.

### Requirement: Account Token availability is scoped to active demand

**Reason**: Its title and scenario names use the retired Account and projection
words, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Token availability is scoped to active demand" in
this capability, with every scenario.

### Requirement: One usable provider is sufficient

**Reason**: Its scenario name says team manifest where the design says team
file, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "One connected provider is sufficient" in this
capability, with every scenario.

## RENAMED Requirements

- FROM: `### Requirement: Explicit host credential policy survives client projection`
- TO: `### Requirement: Explicit host credential policy survives client file writes`
