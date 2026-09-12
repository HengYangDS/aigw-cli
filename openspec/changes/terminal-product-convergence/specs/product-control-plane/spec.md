## ADDED Requirements

### Requirement: Windows path derivation preserves the selected namespace

Derived configuration, data, credential, client-settings and installation paths
SHALL retain the namespace of the explicitly supplied Windows base. Appending
AIGW-owned path components MUST NOT collapse a UNC or extended-length prefix
into a drive-rooted path. Derivation SHALL remain independent of the observer's
operating system and SHALL NOT access a network share to construct its name.

#### Scenario: A user directory is on a network share

- **WHEN** Windows environment paths identify a UNC share or an extended-length
  UNC or drive namespace
- **THEN** every derived AIGW path retains that namespace and base location
- **AND** native Windows path construction agrees with the derived result for
  the supported base forms.

### Requirement: Model discovery reports catalogue evidence only

`aigw catalog` and `aigw models` SHALL share one Account catalogue observation
path. Profile comparison SHALL distinguish listed, not listed, and an unobserved
catalogue with its reason. These commands MUST NOT describe catalogue membership
as inference reachability or native-client readiness. An incomplete or oversized
HTTP response SHALL produce a failed catalogue observation, not partial success.

#### Scenario: A configured model appears in the catalogue

- **WHEN** an authenticated catalogue response lists the configured model ID
- **THEN** `models` reports that ID as listed
- **AND** performs no inference or client invocation.

#### Scenario: A model is absent from a successfully observed catalogue

- **WHEN** a complete catalogue does not list the configured model ID
- **THEN** `models` reports not listed rather than unavailable.

#### Scenario: Catalogue observation fails

- **WHEN** credentials cannot be observed, the endpoint is absent, the request
  fails, or the response is incomplete or oversized
- **THEN** the affected Account has no established model membership
- **AND** its observation reason remains visible without blocking other Accounts.

### Requirement: Control-plane convergence is client-scoped and monotonic

AIGW SHALL derive operational state only from Accounts, client-scoped Profiles,
explicit per-client Routes, admitted client Adapters, and the selected
credential backend. Setup, selection, synchronization, and readiness MUST NOT
depend on a global Profile, an aggregate selection flag, another client's
Route, or the presence of an external compatibility product.

#### Scenario: Both clients are selected independently

- **WHEN** an operator selects one Codex Profile and one Claude Profile in
  separate operations
- **THEN** both per-client Routes remain selected
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

- **WHEN** a valid selected Route belongs to a client that is not installed
- **THEN** AIGW records that capability as deferred
- **AND** the installed client's independent Route remains usable.

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
- **AND** cancellation returns from the identity-migration service independently
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

### Requirement: Extension preserves the control-plane core

An ordinary Provider SHALL be added through Account, endpoint, Profile, Route,
and optional diagnostic declarations. A new client SHALL be added through one
admitted client Adapter and its conformance fixtures. Neither extension SHALL
introduce Provider-name branching in the core, reuse another client's
projection, or make an optional product a dependency.

#### Scenario: Add a client-native cloud model service

- **WHEN** an AWS model service exposes an admitted client protocol and the
  client already owns its credential chain and request signing
- **THEN** its Account and Profiles use the ordinary manifest path
- **AND** the Profile declares client-native authentication
- **AND** AIGW neither stores the cloud credential nor implements a signing
  Adapter.

#### Scenario: Add another agent client

- **WHEN** Hermes, OpenCode, Pi, Qoder, or another client is admitted
- **THEN** it supplies its own discovery, projection, credential, rollback,
  verification, and uninstall contract
- **AND** existing Provider data and client Adapters remain unchanged.

#### Scenario: Supply operational adapters in a different order

- **WHEN** the registry receives operational adapters in an order different
  from its admitted client declarations
- **THEN** discovery, planning and all-client application SHALL follow admission
  order, independently of constructor argument order
- **AND** a failed application SHALL compensate completed adapters in reverse
  application order.

#### Scenario: An adapter's compensation fails

- **WHEN** application fails after prior adapters have completed and one prior
  adapter reports a compensation failure
- **THEN** the registry SHALL still attempt every other prior adapter's
  compensation in reverse application order
- **AND** the returned error SHALL preserve the application failure and each
  compensation failure without claiming successful restoration.

### Requirement: Client deactivation is ownership-bounded

AIGW SHALL withdraw client integration through the same guarded projection
transaction that created it. Disabling one Adapter SHALL remove only that
client's AIGW-owned configuration block, sidecar, generated catalogue, and
credential-helper or command projection. Portable uninstall SHALL first disable
every enabled Adapter and SHALL remove the executable only after client
withdrawal succeeds. Both operations SHALL preserve Accounts, Profiles, Routes,
Tokens, other enabled Adapters, and neighboring user-authored client state.

A successful disable or uninstall SHALL remove the verified checkpoint because
it describes client projections that are no longer present. The single previous
configuration backup MAY remain as an explicit operator-selected rollback
source; it MUST NOT be treated as current state or applied implicitly.

#### Scenario: Disable one client

- **WHEN** an operator disables one enabled client Adapter
- **THEN** only that client's AIGW-owned projection and ownership state are
  withdrawn
- **AND** the other client, capability configuration, credentials, and
  user-authored client settings remain unchanged
- **AND** the stale verified checkpoint is absent.

#### Scenario: A user extends settings created by AIGW

- **GIVEN** AIGW created a previously absent client settings file
- **AND** the user subsequently added fields outside AIGW's ownership
- **WHEN** the Adapter is disabled or the portable program is uninstalled
- **THEN** those user fields remain and only AIGW-owned fields are withdrawn
- **AND** the file is deleted only when withdrawal leaves it empty.

#### Scenario: Uninstall the portable program

- **WHEN** an operator uninstalls AIGW with one or more enabled client Adapters
- **THEN** every AIGW-owned client projection is withdrawn before the executable
  and its program rollback copy are removed
- **AND** capability configuration, credentials, neighboring user state, and the
  explicit previous-configuration backup remain available
- **AND** no verified checkpoint continues to claim the withdrawn projections.

### Requirement: Token rotation is credential-scoped

AIGW SHALL validate and replace only the selected Account's Token. Rotation
MUST leave Accounts, Profiles, Routes, client configuration, ownership sidecars
and client-owned credentials unchanged. Setup, service creation and rotation SHALL use the same
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

### Requirement: Service creation is an atomic client-scoped transition

Service creation SHALL admit a new Account and first Profile before requesting
its Token. It SHALL select that Profile and reconcile only its client's
projection through the shared synchronization transaction. An existing Account
or Profile identity SHALL NOT be implicitly replaced.

#### Scenario: Add and select another service

- **WHEN** service creation succeeds for an installed admitted client
- **THEN** the stored Route and that client's configuration SHALL select the
  new service without requiring a second selection or synchronization command
- **AND** unrelated client configuration and ownership state SHALL be unchanged,
  including unrelated external edits.

#### Scenario: Creation admission fails

- **WHEN** the Account or Profile already exists, desired configuration is
  invalid, or the request is cancelled before acquisition
- **THEN** creation SHALL reject the request before requesting a Token or
  changing configuration, credentials or client projections.

#### Scenario: Service creation cannot complete synchronization

- **WHEN** service creation stores its Token but configuration persistence or
  selected-client projection fails
- **THEN** it SHALL compensate through the shared Token replacement owner
- **AND** configuration and projection recovery SHALL use their existing owners
- **AND** it SHALL preserve the original failure and any compensation error
- **AND** failed compensation SHALL NOT be reported as restored storage.

### Requirement: Verification checkpoints remain bound to current configuration

The configuration Store SHALL own checkpoint admission, bounded mutation-lock
acquisition, guarded persistence, and compensation. Live requests SHALL run
outside that lock. A completed verification SHALL create a checkpoint only
when its configuration still matches the current configuration. A checkpoint
SHALL NOT create missing configuration or certify missing configuration.

Checkpoint reads SHALL consume exactly one complete JSON document. Checkpoint
creation and reading SHALL use one client-scope contract: a nonempty list of
distinct admitted clients, without requiring every client to have participated.

#### Scenario: Invalid checkpoint content or client scope

- **WHEN** a stored checkpoint includes an additional JSON value or trailing
  malformed content, or its scope is empty, repeated or unadmitted
- **THEN** reading SHALL fail rather than certify the valid prefix
- **AND** creation with invalid scope SHALL preserve configuration, backup and
  the previous checkpoint.

#### Scenario: Configuration changes before verification completes

- **WHEN** a live verification completes after configuration has changed
- **THEN** checkpoint admission fails with an instruction to verify again
- **AND** current configuration and any newer checkpoint remain unchanged.

#### Scenario: An external editor changes configuration during checkpoint commit

- **WHEN** the Store observes changed configuration after checkpoint persistence
- **THEN** it compensates only its unchanged checkpoint postimage
- **AND** it preserves an intervening checkpoint replacement and reports any
  incomplete compensation.

#### Scenario: Checkpoint admission is cancelled

- **WHEN** the command is cancelled while waiting for the mutation lock
- **THEN** checkpoint admission returns the cancellation without a checkpoint
- **AND** the other mutation keeps ownership of its lock.

#### Scenario: A verified recovery snapshot is requested

- **WHEN** a caller requests current configuration with its verified checkpoint
- **THEN** the configuration Store SHALL establish that their persisted meanings
  match before returning verified recovery state
- **AND** formatting-only differences SHALL preserve the exact captured bytes
- **AND** a mismatch SHALL fail without changing configuration, backup or
  checkpoint, without relying on a downstream caller to repeat the check.

### Requirement: Release helpers preserve selected source identity

Release metadata and artifact helpers SHALL use the selected source's origin
and repository. Ambient client defaults SHALL NOT select a different host,
protocol or API subfolder. Existing client credentials and unrelated settings
SHALL remain unchanged.

#### Scenario: GitLab configuration names another API endpoint

- **WHEN** the selected GitLab release source differs from ambient host aliases
  or stored API routing
- **THEN** metadata capture and artifact streaming SHALL reach the selected
  origin and its root API path
- **AND** the unrelated endpoint SHALL receive no request.

#### Scenario: GitHub CLI has an unrelated default host

- **WHEN** an admitted GitHub release uses the authenticated CLI fallback
- **THEN** metadata and downloads SHALL bind the selected source rather than
  the ambient `GH_HOST` value.

#### Scenario: Release coordinates have ambiguous URL meaning

- **WHEN** an origin lacks a hostname or carries a path, query or fragment,
  including an empty query or fragment marker
- **THEN** update and build admission SHALL reject it before network or
  helper execution
- **AND** repository coordinates SHALL retain their unescaped namespace and
  project meaning under native relative-path and URL encoding rules
- **AND** missing namespaces, dot segments or encoded separators SHALL NOT
  reach a Forge.

#### Scenario: A release uses an explicit IPv6 authority

- **WHEN** an HTTPS origin includes a bracketed IPv6 address, port and optional
  root slash with valid repository coordinates
- **THEN** admission SHALL preserve that source identity
- **AND** existing runtime private-HTTP support SHALL remain separate from
  HTTPS-only embedded build metadata.

### Requirement: Local candidate identity is verified before a no-op result

An explicitly selected local portable candidate SHALL pass checksum and
target-layout admission even when its version equals the running version.
Equal version labels SHALL NOT substitute for equal program bytes. Admission
and comparison SHALL preserve the current executable and retained predecessor.

#### Scenario: Reapply the exact current program

- **WHEN** a verified same-version candidate contains the current executable's
  exact bytes
- **THEN** update reports an already-matching program without executing or
  replacing it, changing its predecessor, or consulting a remote source.

#### Scenario: Reapply an unverified or different same-version program

- **WHEN** the archive, checksum, target program, or current executable is
  unavailable or invalid, or the verified candidate's program bytes differ
- **THEN** update fails without reporting a verified no-op
- **AND** current and retained programs remain unchanged.

### Requirement: Repeated portable installation preserves rollback identity

Portable installation SHALL compare executable content before rotating the
retained predecessor. Reinstalling identical bytes SHALL preserve the current
program file and any existing predecessor; it MAY restore executable permissions.
A fresh installation SHALL NOT create a predecessor until different program
bytes replace it.

#### Scenario: Reinstall an unchanged downloaded program

- **GIVEN** a portable executable is installed from another path
- **WHEN** the same executable bytes are installed again
- **THEN** the current program remains in place
- **AND** the predecessor retains its prior bytes or remains absent.

### Requirement: Projection-matched Account Token delivery

Every Account-Token Codex provider SHALL use command-backed authentication
through the absolute AIGW executable. Its helper invocation SHALL carry a
fingerprint of the projected client, Account and endpoint. Before reading a
Token, the helper SHALL compare that fingerprint with the selected Route.
AIGW SHALL neither invoke native login nor modify client-owned credentials.
Client-native authentication SHALL project no AIGW Token helper. Provider
naming and catalogue selection SHALL NOT change credential ownership.

#### Scenario: Account-Token provider projection

- **WHEN** AIGW synchronizes a default or explicit Account-Token provider
- **THEN** the provider table declares `wire_api = "responses"`
- **AND** its auth command invokes the absolute AIGW executable with
  `credential codex <projection-fingerprint>`
- **AND** no Token value is copied into client files.

#### Scenario: Return to the default provider

- **WHEN** a Profile changes from an explicit provider to the default provider
- **THEN** the old attributed provider table is removed transactionally
- **AND** the Profile's declared authentication ownership is preserved.

#### Scenario: A retained projection no longer matches the selected Route

- **WHEN** a helper is invoked after its client, Account or endpoint differs
  from the selected Route
- **THEN** it returns a synchronization and reload instruction
- **AND** reads no Token and writes no credential bytes to standard output.

#### Scenario: Model or label changes without a credential-identity change

- **WHEN** only the selected model or label changes
- **THEN** the existing helper fingerprint still matches
- **AND** the helper reads the same Account's current Token.

#### Scenario: Multiple Codex homes receive the same Account-Token route

- **WHEN** setup discovers multiple admitted, AIGW-owned Codex targets
- **THEN** it projects the matching credential helper into each target
- **AND** reads or stores only the selected Account's Token
- **AND** leaves client-owned credentials unchanged.

### Requirement: Readiness observations remain separated from client execution

AIGW SHALL distinguish local configuration readiness, endpoint observations
and real-client verification. Status SHALL observe selection and owned
projections without reading Token values or invoking native clients. A present
Token, synchronized projection or successful endpoint probe SHALL NOT alone
certify that a real client can authenticate or perform inference.

#### Scenario: A synchronized projection has no live-client evidence

- **WHEN** a client has a valid AIGW-owned projection
- **THEN** status may report that projection as ready
- **AND** does not claim native authentication or inference has been proved.

#### Scenario: Real-client verification is requested

- **WHEN** the operator invokes `aigw verify --for <client>`
- **THEN** the admitted adapter invokes that selected native client
- **AND** reports evidence bounded to the request and client it observed.

### Requirement: Recovery execution and presentation are independent

Synchronization and repair SHALL select writes through `--dry-run` and output
format through `--json` independently. Successful preview and applied results
SHALL expose their execution mode and the same continuation as human output.
Rendering SHALL NOT own transaction commit or compensation.

#### Scenario: Preview a recovery operation as JSON

- **WHEN** `sync` or `repair` succeeds with `--dry-run --json`
- **THEN** output SHALL be one JSON document with `dry_run: true`
- **AND** `next_action` SHALL identify the corresponding apply command
- **AND** configuration, credentials and client projections SHALL remain
  unchanged.

#### Scenario: Apply a recovery operation with JSON output

- **WHEN** `sync` or `repair` succeeds with `--json` without `--dry-run`
- **THEN** the owned projection SHALL be applied
- **AND** output SHALL be one JSON document with `dry_run: false` and
  `next_action: "aigw check"`
- **AND** preview plans SHALL NOT be presented as observed per-target results.

#### Scenario: Output fails after a successful recovery commit

- **WHEN** a recovery operation commits and its result cannot be written
- **THEN** it SHALL return the output error and preserve the committed state
- **AND** generic presentation SHALL NOT claim that rollback occurred or will
  occur.

### Requirement: Invocation output has one owner

The CLI root SHALL finalize command output once, preserving command, output and
cleanup failures. Generic JSON errors SHALL use the same Problem and continuation
as human errors. Credential-helper stdout SHALL contain only credential output.

#### Scenario: A JSON-capable command fails before writing a result

- **WHEN** the selected command has parsed `--json` as true and fails before
  writing its result
- **THEN** stdout SHALL contain one JSON document with `ok: false`, `error` and
  `next_action`, preserving available evidence and impact
- **AND** the command SHALL return a nonzero exit status.

#### Scenario: Output format follows the parsed command

- **WHEN** `--json=false` is supplied, or `--json` appears after the `--`
  argument separator
- **THEN** the generic error SHALL use human output
- **AND** command-resolution failures before flag parsing SHALL use human output.

#### Scenario: Output cannot be completed

- **WHEN** the result writer fails or accepts only part of a write
- **THEN** the invocation SHALL preserve the write failure and any command error
- **AND** SHALL NOT append another result to the incomplete output
- **AND** a subsequent invocation SHALL start with independent output state.

#### Scenario: Credential helper fails

- **WHEN** the credential helper fails command validation or execution
- **THEN** the executable SHALL return a nonzero exit status without rendering
  a diagnostic on credential stdout.

### Requirement: Command help preserves native metadata

Command help SHALL derive grammar, descriptions, examples and option metadata
from the command tree. pflag SHALL own flag notation, value types and declared
defaults. Help SHALL remain available without configuration or credential access.

#### Scenario: Inspect detailed command help

- **WHEN** a command declares detailed text, examples, typed flags, defaults or
  inherited options
- **THEN** help SHALL expose those declarations in the existing terminal layout
- **AND** defaults SHALL reflect declarations rather than previously parsed values
- **AND** narrow output SHALL preserve the same option meaning.

#### Scenario: Inspect any command before setup

- **WHEN** any command is invoked with `--help` before configuration exists
- **THEN** help SHALL show its native option metadata without creating
  configuration storage or invoking its operation.

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

- **WHEN** service or Profile creation supplies an invalid identifier, omits
  required client/model/Account flags, supplies blank required values, selects
  an inadmitted client, or configuration import supplies a blank manifest path
- **THEN** the command SHALL reject the invocation with actionable guidance
  before creating configuration storage or accessing credentials.

### Requirement: Program replacement has an explicit client reconciliation boundary

Successful program update or rollback SHALL direct the operator to execute
`aigw sync` with the newly active executable before checking readiness. Program
replacement SHALL NOT silently rewrite client settings or convert stored
configuration to a historical schema. The documented rollback journey SHALL
withdraw enabled integrations before activating a predecessor and recreate
them through that predecessor's public commands.

#### Scenario: A replacement changes the credential helper contract

- **WHEN** an operator activates another program version
- **THEN** the active program's explicit synchronization SHALL reconcile its
  AIGW-owned client projection before the operator resumes client work
- **AND** lifecycle acceptance SHALL execute the projected helper rather than
  infer its arguments from the candidate implementation.

### Requirement: Online update owns its temporary resources and preserves failure causes

An online update SHALL own all peer download directories within one operation
workspace and attempt to remove that workspace on every return. Cleanup SHALL
preserve unrelated files and SHALL NOT alter the completed installation outcome.
Reported failures SHALL retain their original causes for caller inspection.

#### Scenario: A release peer is temporarily unavailable

- **WHEN** a configured peer fails during metadata or asset transport
- **THEN** the updater SHALL apply the same unavailability classification for
  GitLab and GitHub and continue with another admitted peer
- **AND** if every peer is unavailable, the error SHALL retain every peer's cause
- **AND** authentication or integrity failures SHALL remain terminal.

#### Scenario: Workspace cleanup fails

- **WHEN** the operation's workspace cannot be fully removed
- **THEN** the error SHALL identify that exact workspace and preserve the cleanup
  cause together with any preceding failure
- **AND** if replacement completed, the diagnostic SHALL state that the program
  was updated rather than implying that installation was unchanged or rolled back.

### Requirement: Release construction owns only its operation resources

Release construction and native acceptance SHALL retain tool and cleanup failure
causes and identify the exact workspace requiring cleanup. Replacing release
output SHALL use an operation-owned private backup rather than deleting a
predictably named sibling of the requested output.

#### Scenario: Existing output has an unrelated neighboring file

- **WHEN** a release replaces an existing output directory
- **THEN** files outside that directory and the operation's private workspace
  SHALL remain unchanged, regardless of their names
- **AND** failed publication SHALL restore the prior output; if restoration
  also fails, the error SHALL retain both causes and identify the preserved backup.

#### Scenario: Output was published but predecessor cleanup failed

- **WHEN** the new output is installed and removal of its private backup fails
- **THEN** the result SHALL report completed publication and the cleanup error
- **AND** the retained predecessor SHALL remain discoverable at the reported path.

### Requirement: Peer ancestry observation preserves local reference state

Publication ancestry checks SHALL consume the exact observed peer commit and
SHALL preserve existing Git references and `FETCH_HEAD`. An already available
commit requires no transport; fetching a missing object SHALL create no
temporary branch or observation reference.

#### Scenario: Compare an observed peer commit with the publication source

- **WHEN** both commits are available
- **THEN** the comparison SHALL distinguish ancestry from genuine divergence
- **AND** transport or Git execution failures SHALL remain errors rather than
  being interpreted as authorization to force a divergent update.

### Requirement: Catalogue observation owns an isolated and bounded probe lifetime

Bundled and effective catalogue readers SHALL execute with an operation-owned
client home and bounded subprocess lifetime. They SHALL preserve the operator's
configuration and report cleanup failures with the exact owned path and original
failure causes. Repository verification SHALL apply the same ownership rule to
its generated catalogue input.

#### Scenario: Client catalogue retrieval fails after identifying the executable

- **WHEN** the client reports its version but catalogue retrieval fails
- **THEN** the bundled reader SHALL retain the measured executable identity and
  original command failure
- **AND** successful cleanup SHALL remove the isolated home; failed cleanup
  SHALL add its cause without discarding the command failure or measured identity.

### Requirement: Failure recovery retains owned-resource cleanup outcomes

Credential writes, explicit client verification and coverage execution SHALL
retain their original failure when resource cleanup also fails. Cleanup SHALL
name the exact owned resource and SHALL NOT silently report success.

#### Scenario: A credential replacement fails before publication

- **WHEN** replacement cannot rename its staging file and staging removal also fails
- **THEN** the original credential SHALL remain unchanged and both failure causes
  SHALL be observable with the exact retained staging name
- **AND** once replacement commits, cleanup SHALL NOT remove a subsequently
  recreated file at the old staging name.

#### Scenario: Client verification creates additional output

- **WHEN** a client verification command writes into its working directory
- **THEN** that directory SHALL be private to the verification operation and
  reclaimed on success or failure
- **AND** cleanup failure SHALL retain the invocation error and exact workspace.

#### Scenario: Coverage measurement succeeds but profile cleanup fails

- **WHEN** a coverage command cannot remove its owned temporary profile
- **THEN** its exit status SHALL indicate failure and its diagnostic SHALL name
  that file without suppressing any preceding test failure.

#### Scenario: A guarded file write fails during staging

- **WHEN** writing, setting permissions or syncing an owned temporary file fails
- **THEN** the commit helper SHALL close its handle exactly once and retain any
  close failure alongside the original error
- **AND** the writer SHALL remove only its failed staging file and report any
  cleanup failure without modifying unrelated files
- **AND** after a successful rename, it SHALL relinquish the old staging name.

## MODIFIED Requirements

### Requirement: Provider-neutral configuration

AIGW SHALL model Accounts, Profiles, Routes, endpoints, authentication
ownership, and native models without provider identity hacks, named gateways,
deployment topology, or global fallback. Each Route MUST bind one admitted
client to one compatible Profile. Only the current schema is executable; old
schemas need replacement. Manifests MUST omit credentials. Readiness SHALL
follow authentication ownership: a bounded Account-Token probe, or local
client-native prerequisites plus `aigw verify`.

Every Profile MUST reference an Account endpoint for its declared client's
protocol, even when no Route selects that Profile. Configuration validation is
the shared admission boundary for local persistence and manifest import; it
does not require a Token, an installed client, or a successful remote request.

Validation SHALL report the first problem in stable order without changing the
input: schema and required collections, Accounts, Profiles, Routes, then
Adapters. Collection keys and credential-like query parameter names SHALL use
lexical order; endpoint diagnostics SHALL check Anthropic before Responses.

#### Scenario: Diagnose several configuration problems

- **WHEN** the same configuration contains several invalid entries
- **THEN** repeated validation SHALL identify the same first problem
- **AND** the supplied configuration, including unset collections, SHALL remain
  unchanged.

#### Scenario: Import a Profile with an incompatible Account

- **WHEN** a Profile names an admitted client but its Account lacks that client's
  protocol endpoint
- **THEN** configuration validation and manifest import reject it before writes
- **AND** the error identifies the Profile, Account, and missing protocol.

#### Scenario: Save an incompatible Profile

- **WHEN** configuration persistence receives a Profile whose Account lacks its
  declared protocol endpoint
- **THEN** it rejects the change and preserves the existing configuration file.

#### Scenario: Select independent Claude and Codex services

- **WHEN** an operator selects a Claude Profile and a Codex Profile
- **THEN** each client SHALL retain its own explicit Route
- **AND** no global default or implicit inheritance SHALL participate in
  resolution, readiness, or projection.

#### Scenario: Select a Profile without repeating its client

- **WHEN** an operator runs `aigw use <profile>`
- **THEN** AIGW SHALL derive the target client from the Profile's declared
  client
- **AND** SHALL reject a Profile whose client is absent or unadmitted.

#### Scenario: Check every enabled client Route

- **WHEN** Claude and Codex are enabled with distinct selected Routes
- **THEN** `aigw check` SHALL validate both effective Routes
- **AND** SHALL derive each AIGW-owned authentication request from that Route's
  declared client protocol and authentication mode
- **AND** SHALL not issue an AIGW-owned authentication request for a
  client-native Route
- **AND** SHALL not inspect an unselected historical Profile as a fallback
- **AND** MAY coalesce only authentication probes with an identical Account,
  endpoint, and protocol identity.

#### Scenario: Check a client-native Route

- **WHEN** an enabled Route selects a client-native Profile whose local client
  projection is valid
- **THEN** `aigw check` SHALL report local readiness without claiming that the
  remote authentication or model request succeeded
- **AND** SHALL provide `aigw verify --for <client>` as the explicit live-proof
  continuation.

#### Scenario: No client is enabled

- **WHEN** configuration is valid but no admitted client Adapter is enabled
- **THEN** `aigw check` SHALL report configuration readiness
- **AND** SHALL not claim that an arbitrary gateway or model is healthy.

#### Scenario: Inspect an endpoint address without probing it

- **WHEN** `status --json` reports a resolved Route with an endpoint address
- **THEN** it SHALL report `endpoint_configured` as true
- **AND** SHALL not represent that configuration fact as endpoint or transport
  health.

#### Scenario: Report the exact scope of a successful check

- **WHEN** `check --json` finishes its applicable Route checks
- **THEN** `check_passed` SHALL describe command-check success, not inference
  or real-client readiness
- **AND** a valid client-native projection SHALL remain `configured` without
  an AIGW-owned endpoint diagnostic
- **AND** a configured Account-Token Route with a successful endpoint
  diagnostic SHALL be `endpoint_checked`, not generically `ready`
- **AND** human output SHALL describe the same observed scope
- **AND** `next_action` SHALL carry both repair and verification continuations
  without a duplicate `fix` field.

#### Scenario: Read previous local configuration

- **WHEN** AIGW reads its local configuration
- **THEN** it SHALL accept the current schema with explicit per-client Routes
- **AND** an earlier schema SHALL return its actual version and the current
  required version without inferring or migrating Route selection.

#### Scenario: Import a multi-provider team catalogue

- **WHEN** a team manifest declares recommended Routes for admitted clients
- **THEN** setup SHALL materialize those per-client selections without a
  separate recommended global default
- **AND** a future client SHALL remain unselected until its own Route is
  explicitly admitted.

#### Scenario: Diagnose a partially connected team catalogue

- **WHEN** a reviewed catalogue contains multiple Accounts
- **AND** every Account selected by an enabled client Route has its Token
- **THEN** `aigw doctor` SHALL report the credential state as healthy
- **AND** SHALL NOT fail for an unselected Account whose Token is absent.

#### Scenario: Diagnose a selected Account without a Token

- **WHEN** an enabled client Route selects an Account-Token Profile whose Token
  is absent
- **THEN** `aigw doctor` SHALL report that Account as unhealthy
- **AND** SHALL provide the account-scoped rotation action.

#### Scenario: Diagnostic outcome is independent of presentation

- **WHEN** `doctor` observes a failed diagnostic or an invalid, degraded,
  unavailable or unclassified admitted-client state
- **THEN** human and JSON output SHALL both return a nonzero exit status
- **AND** JSON SHALL contain `ok: false` and the same `next_action` shown to
  the human reader
- **AND** JSON SHALL remain one document without appended prose or another
  error rendering
- **AND** successfully writing that report SHALL NOT change the diagnosis.

#### Scenario: Deferred clients do not invalidate a sound configuration

- **WHEN** all diagnostics pass and the observed clients are configured,
  endpoint-checked or intentionally deferred
- **THEN** `doctor` SHALL return success in human and JSON modes
- **AND** SHALL not invent a repair continuation.

#### Scenario: Diagnose a client-native Profile

- **WHEN** an enabled client Route selects a client-native Profile
- **THEN** `status`, `check`, `doctor`, `test`, `credential`, and Profile
  inspection SHALL NOT query the AIGW Account Token store for that Profile
- **AND** read-only output SHALL identify client-owned authentication and use
  `aigw verify --for <client>` for live proof where applicable.

#### Scenario: Add an ordinary provider

- **WHEN** an operator imports token-free Account and Profile data for a new
  endpoint
- **THEN** Codex or Claude Code SHALL select it without a provider-specific CLI,
  installer, projection branch, service manager, or core dependency.

#### Scenario: A Responses endpoint needs compatibility behavior

- **WHEN** an endpoint needs storage, replay, or other wire compatibility
- **THEN** AIGW SHALL NOT rename its provider identity or encode transport
  behavior in Account metadata.

#### Scenario: Reject implicit credential transport

- **WHEN** an imported manifest contains a token, password, authorization
  header, API key, or equivalent credential field
- **THEN** AIGW SHALL reject the manifest without changing local configuration.

### Requirement: Profile-scoped Codex native provider

A Profile SHALL remain the sole owner of its client, Account, model, and
optional client-native provider selection. A missing Codex provider selection
SHALL resolve to the canonical `aigw` provider. Provider selection SHALL NOT be
inferred from an Account, endpoint, proxy implementation, or client-private
state. Authentication ownership SHALL be independent of the provider name.

#### Scenario: Explicit Codex provider

- **WHEN** a Codex-scoped Profile declares a safe `model_provider`
- **THEN** its resolved Runtime carries that exact provider identity
- **AND** Codex receives one attributed provider table using the Profile's
  Account endpoint and declared authentication mode.

#### Scenario: Default Codex provider

- **WHEN** a Codex-scoped Profile omits `model_provider`
- **THEN** its Runtime resolves the canonical `aigw` provider
- **AND** Account-Token authentication uses the same command-helper contract
  as an explicit provider.

#### Scenario: Provider ownership is narrow

- **WHEN** a non-Codex Profile or an unsafe provider identifier declares
  `model_provider`
- **THEN** configuration validation fails before persistence or projection.

## REMOVED Requirements

### Requirement: Provider-owned Codex authentication

**Reason**: Provider names must not select credential ownership. Native-login
writes duplicated AIGW Account Tokens into client-owned storage beyond AIGW's
compensation boundary.

**Migration**: Synchronize AIGW-owned Account-Token projections to the scoped
command helper and reload the client. Preserve existing client-owned
credentials. Client-native Profiles retain their own credential mechanism.

### Requirement: Client readiness is evidence-bounded

**Reason**: Generic login-status evidence does not describe command-backed
provider authentication. The replacement separates local projection readiness,
endpoint observations and real-client execution.

**Migration**: Consume status for local configuration, check for endpoint
observations, and verify for evidence from the selected client. Retire the
native-authentication status field and adapter-auth command.
