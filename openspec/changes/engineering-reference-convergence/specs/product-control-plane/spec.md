## REMOVED Requirements

### Requirement: Service creation is an atomic client-scoped transition

**Reason:** The name treats a Provider service as the created domain object,
while the operation actually creates one Account and its first Profile.

**Migration:** Use the `Account connection is an atomic client-scoped
transition` requirement and the `aigw add <account>` command.

## ADDED Requirements

### Requirement: Model Profiles are reusable across compatible clients

A Profile SHALL identify an Account and an actual upstream model independently
of client branding. A reviewed catalogue Profile SHALL also declare the wire
protocols verified for that exact Account and model; manually authored Profiles
without this metadata retain explicit-binding compatibility but SHALL NOT be
presented as verified catalogue entries. Each client binding SHALL select a
Profile and own its explicit enabled intent, native target and client-specific
options. The binding SHALL replace parallel Route and Adapter selection as the
operational authority. The intersection of the Profile's verified protocols,
the Account endpoints, and the selected client's supported interfaces SHALL
determine protocol compatibility without model-name inference.

#### Scenario: One model is selected by two clients

- **GIVEN** an Account exposes interfaces compatible with both clients
- **WHEN** the operator selects the same Profile for both clients
- **THEN** there SHALL be one reusable Profile and two independent client bindings
- **AND** changing one binding SHALL preserve the other client's choice and files.

#### Scenario: More than one endpoint is compatible

- **WHEN** a binding has multiple compatible endpoints and no explicit choice
- **THEN** selection SHALL request a protocol choice before mutation
- **AND** endpoint order, model prefixes and unrelated client settings SHALL NOT
  decide the selection.

#### Scenario: One Account exposes different protocols per model

- **GIVEN** an Account exposes Responses and Chat Completions endpoints
- **AND** two reviewed Profiles on that Account were verified on different protocols
- **WHEN** a client resolves either Profile
- **THEN** only that Profile's verified protocol set SHALL be eligible
- **AND** Account-level endpoint presence SHALL NOT imply model-level compatibility.

#### Scenario: The team curates broad model choice

- **WHEN** the reviewed team catalogue admits a general model family
- **THEN** it SHALL normally identify one `flagship` and one `daily` Profile per Account
- **AND** the tier SHALL remain display metadata rather than a routing or capability inference.

#### Scenario: The operator disables a client

- **WHEN** a client is disabled and its owned projection is withdrawn
- **THEN** synchronization, discovery and team import SHALL retain disabled intent
- **AND** the reusable Profile, credential reference and other clients remain intact.

#### Scenario: A supported but unused client is discovered

- **WHEN** the operator inspects an otherwise healthy configured installation
- **THEN** an unselected client SHALL NOT create a health failure or mandatory
  setup action
- **AND** optional availability SHALL remain discoverable separately.

### Requirement: Schema replacement is explicit and reversible

AIGW SHALL migrate the prior supported configuration through a bounded explicit
operation with a preview of Accounts, equivalent Profiles, client bindings and
native options. Normal runtime SHALL interpret only the current schema. The
migration SHALL preserve explicit choices, disabled intent, credentials in their
authoritative stores, unrelated client settings and existing session metadata.

#### Scenario: The migration preview has not been applied

- **WHEN** the operator previews the prior configuration's canonical replacement
- **THEN** no configuration, credential, client projection or session SHALL change
- **AND** ambiguous identity or ownership SHALL be reported before application.

#### Scenario: State changes after migration preparation

- **WHEN** any affected preimage differs before its prepared write
- **THEN** migration SHALL preserve that state and report the conflict
- **AND** already applied owned writes SHALL use guarded compensation.

#### Scenario: The new schema is accepted

- **WHEN** retained-state migration and native candidate acceptance pass
- **THEN** old normal-runtime readers and duplicate selection state SHALL be removed
- **AND** rollback SHALL use the immutable predecessor and matching retained state,
  not a second live schema authority.

### Requirement: Requested client surfaces have explicit adapters

AIGW SHALL integrate Hermes and Claude Desktop through their supported native
configuration and credential interfaces. Each Adapter SHALL use the shared
Client Binding, transaction, discovery, verification, and withdrawal owners. Claude
Desktop Chat, Cowork, and Code SHALL retain distinct capability evidence.
Client-owned sessions, services, and model choices SHALL remain client-owned.

#### Scenario: Hermes becomes available after team import

- **GIVEN** team configuration was imported before Hermes was installed
- **WHEN** Hermes becomes available and its Profile is explicitly selected
- **THEN** synchronization SHALL configure its admitted provider and model
- **AND** credentials SHALL use the selected supported delivery mechanism
- **AND** unrelated clients, Hermes sessions, and Hermes services remain unchanged.

#### Scenario: Hermes exposes the connected catalogue

- **GIVEN** more than one Account has an available credential
- **WHEN** Hermes synchronization projects its native model catalogue
- **THEN** every reviewed Profile compatible with Hermes from those connected
  Accounts SHALL be grouped under its Account and protocol provider
- **AND** the explicitly selected Profile SHALL remain Hermes' active model
- **AND** disconnected Accounts and unverified manual Profiles SHALL remain absent.

#### Scenario: Claude Desktop and Claude Code use different bindings

- **WHEN** the operator enables distinct Desktop and CLI Profiles
- **THEN** each surface SHALL use its own documented configuration boundary
- **AND** status SHALL state which Desktop modes and platforms were qualified
- **AND** a required app restart SHALL be reported before activation is claimed.

### Requirement: Model admission follows protocol and capability

AIGW SHALL admit model identifiers independently of client branding, using the
selected endpoint's protocol and verified capabilities. Provider-native APIs
SHALL be preferred when sufficient. An independently selected compatibility
service MAY supply a missing protocol without becoming part of AIGW's runtime.
Displayed mappings SHALL preserve the identity of the requested and serving
models. A successful text request SHALL establish only text connectivity.

#### Scenario: A non-Anthropic model serves Claude

- **WHEN** an explicit Client Binding selects a non-Anthropic model through a compatible
  Messages endpoint
- **THEN** AIGW SHALL preserve that provider and model identity in its output
- **AND** support claims SHALL reflect tested streaming, tool results,
  continuation, and model-specific limits.

#### Scenario: Codex uses an independent Responses endpoint

- **WHEN** an explicit Client Binding selects a model through an independent Responses
  endpoint
- **THEN** validation SHALL verify its actual input, event, and tool contracts
- **AND** missing compaction or hosted tools SHALL have explicit outcomes
- **AND** existing conversation metadata and native model selections remain unchanged.

#### Scenario: A provider exposes a smaller capability set

- **WHEN** an endpoint does not support a capability required by a client journey
- **THEN** the journey SHALL report the specific unsupported capability
- **AND** configuration or a renamed model SHALL NOT be presented as full acceptance.

### Requirement: Account connection is an atomic client-scoped transition

Account connection SHALL admit a new Account and first Profile before requesting
its Token. It SHALL select that Profile and reconcile only its client's
projection through the shared synchronization transaction. An existing Account
or Profile identity SHALL NOT be implicitly replaced.

#### Scenario: Add and select another Account

- **WHEN** Account connection succeeds for an installed admitted client
- **THEN** the stored Client Binding and that client's configuration SHALL select the new
  Account Profile without requiring a second selection or synchronization command
- **AND** unrelated client configuration and ownership state SHALL be unchanged,
  including unrelated external edits.

#### Scenario: Creation admission fails

- **WHEN** the Account or Profile already exists, desired configuration is
  invalid, or the request is cancelled before acquisition
- **THEN** creation SHALL reject the request before requesting a Token or
  changing configuration, credentials or client projections.

#### Scenario: Account connection cannot complete synchronization

- **WHEN** Account connection stores its Token but configuration persistence or
  selected-client projection fails
- **THEN** it SHALL compensate through the shared Token replacement owner
- **AND** configuration and projection recovery SHALL use their existing owners
- **AND** it SHALL preserve the original failure and any compensation error
- **AND** failed compensation SHALL NOT be reported as restored storage.

## MODIFIED Requirements

### Requirement: Control-plane convergence is client-scoped and monotonic

AIGW SHALL derive operational state from Accounts, reusable model Profiles,
explicit client bindings, admitted native Adapters and the selected credential
backend. A binding owns the client selection and enabled intent; there SHALL
NOT be a second independently persisted selection. Setup, selection, synchronization, and readiness MUST NOT
depend on a global Profile, an aggregate selection flag, another client's
binding, or the presence of an external compatibility product.

Discovery SHALL report client availability without creating native files,
selecting a Profile, or changing enabled intent. A client that has not created
its native state SHALL remain untouched until the operator explicitly binds it.
Every projection SHALL preserve unrelated native fields and reject a changed
preimage rather than overwrite concurrent user or tool edits.

#### Scenario: A client is discovered but not configured

- **WHEN** discovery finds an admitted executable whose native state has not
  been initialized or explicitly bound
- **THEN** AIGW SHALL report it as available without creating its configuration
- **AND** synchronization and readiness of other clients remain unaffected.

#### Scenario: Both clients are selected independently

- **WHEN** an operator selects one Codex Profile and one Claude Profile in
  separate operations
- **THEN** both per-client bindings remain selected
- **AND** readiness requires no additional aggregate selection operation.

#### Scenario: An ordinary configuration edit affects one client

- **WHEN** an Account or Profile edit changes one client's persistent projection
- **THEN** the configuration transaction SHALL plan and apply only the affected
  client set in admission order
- **AND** another client's external edits and ownership state SHALL remain
  byte-identical, without being inspected as a prerequisite for that edit.

#### Scenario: A shared edit affects multiple clients

- **WHEN** an edit changes several client projections
- **THEN** all affected clients SHALL participate in the same guarded
  transaction and existing reverse compensation contract
- **AND** explicit synchronization or repair SHALL still reconcile its entire
  requested client scope, including unchanged configuration.

#### Scenario: One client is absent

- **WHEN** a valid selected Client Binding belongs to a client that is not installed
- **THEN** AIGW records that capability as deferred
- **AND** the installed client's independent binding remains usable.

#### Scenario: An external compatibility endpoint is absent

- **WHEN** no local compatibility service is installed
- **THEN** AIGW remains usable with any configured native HTTPS endpoint
- **AND** no external-gateway file, process, service, port, or lifecycle state
  is required.

#### Scenario: Cancellation precedes mutation admission

- **WHEN** setup, configuration selection, projection, or repair observes a
  cancelled request at its mutation boundary
- **THEN** it returns cancellation without starting that boundary's writes
- **AND** setup restores any Token already prepared before cancellation was
  observed, subject to the existing credential ownership guard.

#### Scenario: Preparation observes cancellation before persistence

- **WHEN** snapshot capture or projection preflight completes with the request
  cancelled
- **THEN** configuration persistence does not start
- **AND** existing configuration, backup, checkpoint and client files remain
  unchanged.

#### Scenario: Cancellation prevents a later client projection

- **WHEN** cancellation is observed before the next client adapter starts
- **THEN** that adapter performs no writes
- **AND** the registry compensates prior adapters in reverse order and the
  enclosing transaction compensates its configuration
- **AND** cancellation and any recovery conflicts remain inspectable errors.

#### Scenario: Cancellation follows the last admitted projection

- **WHEN** the last admitted adapter completes successfully after cancellation
  arrives during its synchronous operation
- **THEN** the completed transaction remains successful
- **AND** no post-completion cancellation check undoes its committed result.

#### Scenario: Identity migration is cancelled before writes

- **WHEN** Account renaming or verified finalization observes cancellation
  before credential preparation or finalization admission
- **THEN** configuration, backup and source and target credentials remain
  unchanged
- **AND** cancellation returns from the identity-migration owner independently
  of the CLI; credential copies already admitted before a later configuration
  commit failure retain both slots for retry and rollback.

#### Scenario: Setup is requested for an existing installation

- **GIVEN** the current configuration already contains Profiles
- **WHEN** interactive, explicit, manifest or internal setup is requested
- **THEN** the shared setup admission rejects it before prompting, Provider
  probing, client discovery or mutation
- **AND** configuration import, selection and Token rotation retain their
  separate operations rather than becoming implicit setup behavior.

#### Scenario: First-time setup finds an existing credential slot

- **GIVEN** the current configuration has no Profiles but an Account Token is
  already available
- **WHEN** first-time setup is requested
- **THEN** that Token does not by itself classify the installation as configured
- **AND** setup may use or explicitly replace it with guarded compensation.

### Requirement: Token rotation is credential-scoped

AIGW SHALL validate and replace only the selected Account's Token. Rotation
MUST leave Accounts, Profiles, Client Bindings, client configuration, ownership
sidecars and client-owned credentials unchanged. Setup, Account connection and
rotation SHALL use the same
Token replacement and compensation owner. Compensation SHALL inspect the
written Token, preserve a different observed value, and report incomplete
storage recovery.

#### Scenario: The next helper invocation observes rotation

- **WHEN** rotation successfully replaces the selected Account's Token
- **THEN** a matching projected helper reads the replacement on its next call
- **AND** rotation invokes no client and changes no client-owned credentials
- **AND** its result does not promise immediate refresh by a running client.

#### Scenario: A newer Token appears before compensation

- **WHEN** compensation observes a Token different from the one it wrote
- **THEN** it preserves the observed Token and reports the ownership conflict
- **AND** client-owned credentials remain unchanged.

#### Scenario: The operator cancels validation

- **WHEN** the rotation command is cancelled during Token validation
- **THEN** validation observes the command cancellation
- **AND** no replacement Token is persisted.

### Requirement: Argument admission precedes mutation

The CLI SHALL validate positional arguments, required flags, flag relationships
and command-specific argument shape before acquiring a configuration mutation
lock or executing an operation. Cobra SHALL own shared flag validation.

#### Scenario: Select a local update with invalid paths or options

- **WHEN** an explicitly supplied candidate or checksum path is empty or
  whitespace-only, its paired option is missing, or rollback is combined with
  candidate options
- **THEN** the command SHALL reject the invocation before creating configuration
  storage or calling an online update, candidate update or rollback operation.

#### Scenario: Supply invalid manifest setup arguments

- **WHEN** the manifest path or explicitly selected Account ID is blank,
  manifest mode is combined with single-profile options, JSON output is
  requested without manifest mode, or manifest-mode Token input has no Account
  owner
- **THEN** setup SHALL reject the invocation before creating configuration
  storage, reading the manifest, or executing setup.

#### Scenario: Supply incomplete account finalization intent

- **WHEN** account finalization lacks explicit old/new identifiers, or credential
  rotation confirmation is requested without finalization
- **THEN** argument admission SHALL reject the invocation before acquiring a
  configuration lock or creating configuration storage
- **AND** the diagnostic SHALL direct the operator to that command's help,
  rather than an unrelated readiness check.

#### Scenario: Supply invalid rename arguments

- **WHEN** noninteractive Account or Profile rename omits an identifier, an
  explicitly supplied identifier violates the configuration identifier contract,
  or an incomplete interactive invocation has no prompt capability
- **THEN** argument admission SHALL reject it before configuration access,
  mutation-lock acquisition, credential access or discovery
- **AND** the diagnostic SHALL direct the operator to the invoked command's help
- **AND** complete explicit identifiers SHALL require no prompt, while guided
  rename SHALL remain available when interactive input can complete the request.

#### Scenario: Supply invalid selection or adapter arguments

- **WHEN** noninteractive selection omits its Profile, an adapter client is not
  admitted, its executable is missing or blank, or a required target is missing
  or an explicitly supplied target is blank
- **THEN** the command SHALL reject the invocation with an actionable repair
  command before creating configuration storage
- **AND** it SHALL NOT invoke credential access, client discovery or mutation.

#### Scenario: Supply invalid creation or import arguments

- **WHEN** Account or Profile creation supplies an invalid identifier, omits
  required client/model/Account flags, supplies blank required values, selects
  an inadmitted client, or configuration import supplies a blank manifest path
- **THEN** the command SHALL reject the invocation with actionable guidance
  before creating configuration storage or accessing credentials.

#### Scenario: Supply invalid metadata edit intent

- **WHEN** Account or Profile editing supplies no editable flag, an invalid
  identifier or a blank label or endpoint, Profile removal supplies an invalid
  identifier, or diagnostic-credential enablement lacks an interactive terminal
- **THEN** admission SHALL reject the invocation before configuration access or
  creation, mutation-lock acquisition or credential access
- **AND** native flag-group failures SHALL identify the invoked command's help,
  not an unrelated readiness check
- **AND** explicit empty Profile purpose SHALL remain a valid request to clear
  that optional display field.
