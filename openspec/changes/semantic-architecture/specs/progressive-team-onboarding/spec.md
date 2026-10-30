## ADDED Requirements

### Requirement: Reviewed team configuration is directly consumable

The repository SHALL publish one token-free reviewed manifest directly
consumable by `aigw setup --from` without credentials or installed clients.
Setup and sync SHALL preserve existing Client Bindings and fill only unselected
bindings from declared primary Routes and ordered alternatives with usable
authentication; any other Route SHALL require explicit selection. They SHALL
project only AIGW-owned state and never expose or rebind Tokens.

#### Scenario: The shipped manifest stays reviewed

- **WHEN** the repository ships its team manifest
- **THEN** fictitious providers, workstation paths, and parallel example
  manifests SHALL NOT remain
- **AND** a Route outside the recommendations SHALL require explicit selection
  even if its Account is connected or its Model matches.

#### Scenario: Team member imports reviewed settings

- **WHEN** a team member downloads the tracked manifest and runs `aigw setup --from`
- **THEN** AIGW SHALL import the Accounts and Routes from the reviewed
  `manifests/team.toml` without a second provider or model-name policy
- **AND** required Account Tokens SHALL remain outside the manifest
- **AND** recommended selections SHALL come from that manifest rather than
  duplicated model-version literals in this specification.

#### Scenario: No Account is connected during import

- **WHEN** a user imports the team manifest without supplying a Token
- **THEN** every reviewed Account and Route SHALL be retained
- **AND** no client installation or credential SHALL be required
- **AND** the next action SHALL enumerate the compatible Account connection
  choices without making one Account mandatory.

#### Scenario: One Provider Account is connected

- **WHEN** a user imports the team manifest with exactly one available Account Token
- **THEN** setup SHALL succeed without Tokens for other Accounts
- **AND** each unselected client SHALL select the first usable declared
  recommendation compatible with that client and connected Account
- **AND** an undeclared Route SHALL remain unselected rather than becoming a
  Model or lexical fallback.

#### Scenario: A compatible Account becomes available after import

- **WHEN** setup retained the reviewed catalog without a connected Account
- **AND** a Token for a declared compatible recommendation later becomes
  available through the read-only environment backend
- **THEN** `aigw sync` SHALL select that recommendation for an unselected client
- **AND** SHALL preserve the declared protocol and existing Client Bindings
- **AND** SHALL NOT require Tokens for other Accounts.

#### Scenario: A supported client is installed later

- **WHEN** setup completed before an admitted client was installed
- **AND** its selected Route has usable declared authentication
- **THEN** `aigw sync` SHALL discover and project that client
- **AND** SHALL NOT require, replace, or expose any unrelated Token
- **AND** SHALL leave absent clients untouched.

### Requirement: Account connection is an atomic client-scoped transition

Account connection SHALL admit a new Account and its first canonical Model and
Route before requesting its Token. It SHALL select that Route and reconcile
only its client's projection through the shared synchronization transaction. An
existing Account, Model, or Route identity SHALL NOT be implicitly replaced.

#### Scenario: Add and select another Account

- **WHEN** Account connection succeeds for an installed admitted client
- **THEN** the stored Client Binding and that client's configuration SHALL select the new
  Account Route without requiring a second selection or synchronization command
- **AND** unrelated client configuration and ownership state SHALL be unchanged,
  including unrelated external edits.

#### Scenario: Creation admission fails

- **WHEN** the Account, Model, or Route already exists, desired configuration is
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

### Requirement: Shipped team catalog is a curated capability contract

Team manifests SHALL declare Accounts, canonical Models, provider Routes, and
per-client Recommendations separately. Model and Route IDs SHALL be stable
lower-case; provider wire IDs and channels SHALL remain exact. Route admission
requires authenticated inference and compatible client/protocol evidence.
Setup SHALL need only one compatible Account, preserve explicit Client
Bindings, and exclude inferred Route matrices and proxy endpoints.

#### Scenario: One Account offers a subset of Models

- **WHEN** one connected Account has a compatible recommended Route but lacks
  Routes for other canonical Models or clients
- **THEN** setup SHALL activate only its usable unselected Client Bindings
- **AND** preserve explicit selections and defer unavailable capabilities.

#### Scenario: A provider uses a channel-specific wire ID

- **WHEN** DMXAPI exposes a CC, SSVIP, or CDX channel variant of one Model
- **THEN** that Route SHALL reference the same canonical Model as its base
  Route and carry its exact provider wire ID
- **AND** its lower-case Route ID SHALL not be derived from wire spelling.

#### Scenario: Ordinary Route display needs no duplicate label

- **WHEN** a Route omits `label`
- **THEN** human, JSON, and native-client presentation SHALL derive
  `Account · Model` from the declared labels
- **AND** an explicit channel or user label SHALL override that derivation
- **AND** native manifest export SHALL omit a stored label equal to the
  derived form while preserving an explicit distinct label.

#### Scenario: Account lists follow the declared presentation order

- **WHEN** Accounts declare presentation `order` values
- **THEN** `aigw account list`, human and JSON, SHALL list Accounts by that
  order and then by identity, as Route lists already do
- **AND** serialized configuration and manifest export SHALL keep their
  lexical identity order.

### Requirement: The shipped catalog keeps one general Model per other vendor

The shipped team catalog SHALL hold exactly one general Model for each vendor
other than Anthropic and OpenAI, whose Models are curated per client. That Model
SHALL be the vendor's most capable generally available general model whose
Routes qualify. A qualified successor SHALL replace that Model and its Routes
rather than add a second general Model, and import SHALL keep explicit Client
Bindings to the replaced Routes.

#### Scenario: A vendor's more capable general model becomes available

- **GIVEN** the catalog holds one general Model for a vendor
- **WHEN** that vendor makes a more capable general model generally available
  and its Routes qualify
- **THEN** the catalog SHALL replace that vendor's Model and Routes with the
  successor
- **AND** SHALL NOT keep both as general Models
- **AND** importing the catalog SHALL keep explicit Client Bindings to the
  replaced Routes.

#### Scenario: A newer model is not generally available or not qualified

- **WHEN** a vendor's newer model is limited to selected testers or its Routes
  fail qualification
- **THEN** the catalog SHALL keep that vendor's current Model.

### Requirement: Explicit Route retirement shares the manifest import transaction

Import SHALL retain local-only Routes unless each is named with `--retire-route`.
Retired Routes SHALL exist locally, be absent from the incoming manifest,
and have no Client Binding or retained recommendation. Only their Models
with no remaining Route or incoming declaration SHALL be removed. Account
metadata and Tokens SHALL remain unchanged. Import, retirement, and affected
native projections SHALL share one guarded commit with compensation.

#### Scenario: Reviewed catalog replaces obsolete recommendations

- **WHEN** one import names multiple unselected local Routes for retirement
- **AND** the incoming manifest replaces recommendations that referenced them
- **THEN** the Routes and only their unreferenced, undeclared Models SHALL be
  removed in the same commit as the new public catalog
- **AND** explicit Client Bindings and shared or newly declared Models SHALL
  remain unchanged.

#### Scenario: Retirement is not authorized by identity or selection

- **WHEN** a retirement selector names a missing, incoming-declared, or
  client-selected Route
- **THEN** import SHALL fail before changing configuration, projection, or
  credential entrypoint state.

#### Scenario: Retained recommendation would dangle

- **WHEN** a local recommendation references a Route proposed for retirement
- **AND** the incoming manifest does not replace that recommendation
- **THEN** import SHALL reject the resulting configuration before committing.

## MODIFIED Requirements

### Requirement: Team setup imports capability before activation

`aigw setup --from` SHALL import a credential-free Account and Route
catalog independently from Account connection, Route activation, client
installation, and external endpoint availability. No particular Account,
client, or external Responses service SHALL be mandatory for import. Setup
SHALL connect only the explicitly selected Account or Accounts whose Tokens are
already available through the selected credential backend.

#### Scenario: No Token or client is present

- **WHEN** a user imports a valid team manifest on a host with no admitted
  client and no Account Token
- **THEN** setup SHALL save the catalog without prompting for every Account
- **AND** SHALL report that zero Accounts are connected
- **AND** SHALL show the smallest account-scoped connection command

#### Scenario: One Account is connected

- **WHEN** exactly one manifest Account is explicitly selected or has a locally
  available Token
- **THEN** setup SHALL select that Account's Routes that the manifest declares
  as client recommendations, in declared order
- **AND** a Route of that Account that no recommendation declares SHALL remain
  manual-only
- **AND** other manifest Accounts SHALL remain available but unconnected
- **AND** their missing Tokens SHALL NOT block setup

#### Scenario: Admitted clients are absent

- **WHEN** setup connects an Account before Claude Code or Codex is installed
- **THEN** setup SHALL preserve the connected Account and selected Routes
- **AND** SHALL identify `aigw sync` as the activation action after a client is
  installed

### Requirement: Activation follows present capabilities

Manifest setup SHALL import the reviewed catalog and project only discovered
clients whose selected Routes have locally available declared authentication,
without contacting a provider or running a native client. A connected Token
and a native projection are not claims of accepted authentication, reachable
endpoints, or working inference; `aigw check` owns those observations. A later
synchronization SHALL adopt a newly installed admitted client without manifest
re-import.

#### Scenario: Only Claude Code is installed

- **WHEN** a connected Account has both Anthropic and Responses routes but
  only Claude Code is discovered
- **THEN** setup SHALL configure only the Claude route
- **AND** an unavailable Responses endpoint SHALL NOT block the import.

#### Scenario: Provider is offline during manifest import

- **WHEN** one compatible Account Token is available and its provider cannot
  be reached
- **THEN** manifest setup SHALL retain the catalog, selection, and local
  projection without an online request
- **AND** SHALL NOT report endpoint or inference verification as passed
- **AND** an explicit `aigw check` SHALL report the provider failure without
  exposing the Token.

#### Scenario: Low-level configuration import preserves explicit selection

- **WHEN** `aigw config import` merges reviewed public metadata while a client
  already selects another Route
- **THEN** the selected Route SHALL remain unchanged
- **AND** import SHALL NOT inspect unrelated Account credentials to infer a
  continuation or claim projection readiness
- **AND** it SHALL direct the operator to `aigw status`, which owns the current
  readiness observation and next action.

#### Scenario: Preview a team import against a local endpoint override

- **WHEN** an imported Account shares an ID with a different local Account
  whose endpoint the operator intends to retain
- **THEN** import SHALL reject the conflict unless the operator explicitly
  selects either local retention or incoming replacement for that Account
- **AND** local retention SHALL preserve the complete local Account metadata
  and Token while admitting compatible incoming Models and Routes
- **AND** `--dry-run --json` SHALL apply the same merge validation and report
  explicit Account retention, Route retirement, and semantic client projection
  candidates without reading Tokens or writing configuration or client files
- **AND** a client projection candidate SHALL NOT be described as an executed
  projection or proof that the client or its credential is available.

#### Scenario: Client is installed later

- **WHEN** a manifest was imported before an admitted client was installed
- **AND** the user later runs synchronization after installing that client
- **THEN** AIGW SHALL discover and converge its owned projection
- **AND** SHALL preserve unrelated client and conversation state.

### Requirement: Onboarding state is explicit and actionable

Setup results SHALL distinguish imported capability, connected Accounts,
selected Routes, configured clients, and deferred work, and expose the smallest
safe next action. With environment credentials, setup SHALL list every
compatible variable as an equal choice. With a writable backend, rotation
SHALL preserve Route and Client Binding selection and name the explicit
`aigw use --for` continuation. Manifest setup MUST support `--json`, share one
semantic result, and never expose credentials.

#### Scenario: Guided setup completes before client installation

- **WHEN** guided setup connects an Account for a selected admitted client that
  is not yet installed
- **THEN** the wizard SHALL offer the admitted clients and require an explicit
  protocol choice when the selected client admits more than one
- **AND** setup SHALL preserve the Account, Token, Route, and client selection
  without claiming that native projection or inference is ready
- **AND** SHALL name client installation followed by `aigw sync`, not
  `aigw check`, as the activation continuation.

#### Scenario: Selected Responses endpoint is unavailable

- **WHEN** an installed Codex route selects a Responses endpoint that is not
  reachable
- **THEN** diagnostics SHALL identify the configured endpoint as unavailable
- **AND** SHALL state that AIGW owns configuration rather than endpoint
  lifecycle
- **AND** SHALL offer checking that endpoint or selecting another Responses
  route without inferring the endpoint implementation.

#### Scenario: No Token is available through a writable backend

- **WHEN** a valid team manifest is imported and no Account is connected
- **THEN** setup SHALL recommend `aigw rotate <account>`
- **AND** SHALL NOT imply that every catalog Account Token is required.

#### Scenario: A writable Account Token arrives after import

- **GIVEN** a team catalog with no selected Client Binding
- **WHEN** the operator rotates one compatible Account Token
- **THEN** rotation SHALL leave Route and Client Binding selection unchanged
- **AND** SHALL name an executable `aigw use --for` action for one reviewed
  compatible Route of that Account, rather than immediate `aigw check` or
  no-op synchronization.

#### Scenario: No Token is available through the environment backend

- **WHEN** a valid team manifest is imported with the read-only environment
  backend and no Account Token is available
- **THEN** setup SHALL enumerate the environment variable for every compatible
  Account as an alternative activation choice
- **AND** SHALL direct the operator to run `aigw sync` after setting one
- **AND** SHALL state that any one compatible Account is sufficient.

#### Scenario: Environment Account becomes available later

- **WHEN** a team catalog was imported without a connected Account
- **AND** exactly one compatible Account Token later becomes available through
  the environment backend
- **THEN** `aigw sync` SHALL activate Routes compatible with that Account
- **AND** SHALL NOT require the previously recommended Account or any unrelated
  Account Token.

#### Scenario: Connected Account precedes client installation

- **WHEN** setup or Route selection connects an Account and enables a selected
  Client Binding without recording a native executable for its projection
- **THEN** the Account, Token, Route, and enabled intent SHALL remain distinct
  from the deferred native projection and unverified inference
- **AND** setup, selection, sync preview, status, check, and doctor SHALL name
  installation if needed followed by `aigw sync`, not immediate `aigw check`,
  as the activation continuation
- **AND** check SHALL NOT probe the endpoint before projection is available;
  doctor MAY pass local diagnostics without claiming client readiness.

#### Scenario: Selected Account Token disappears before client installation

- **WHEN** an enabled Client Binding has no native projection and its selected
  Account Token becomes unavailable
- **THEN** observational commands SHALL report the missing Token and deferred
  projection as distinct facts without reading the Token value
- **AND** the next action SHALL restore that selected Account's Token before
  suggesting synchronization or endpoint verification
- **AND** restoring the Token SHALL reveal installation if needed followed by
  `aigw sync` as the remaining activation step.

#### Scenario: Enabled clients have different local readiness

- **WHEN** an Account Token is available, its Codex projection is configured,
  and an enabled Claude binding still lacks a native executable
- **THEN** sync preview, status, check, and doctor SHALL retain both selections
  and give the same installation-then-sync continuation in human and JSON output
- **AND** check MAY probe the configured Codex Route, but SHALL NOT probe the
  unprojected Claude Route or claim all enabled clients passed.

#### Scenario: Manifest setup is consumed by automation

- **WHEN** an operator runs manifest-based setup with `--json`
- **THEN** setup SHALL return the imported Account and Route counts,
  connected Accounts, client states, alternative activation choices, and next
  safe action as machine-readable data
- **AND** SHALL NOT include an Account Token or credential value.

#### Scenario: Setup imports without activation

- **WHEN** no Token or supported client is available
- **THEN** setup reports the catalog as imported
- **AND** does not claim any endpoint, credential, or client is ready.

#### Scenario: Import-only setup has no actionable synchronization

- **WHEN** a valid team catalog is imported with no compatible Token and no
  admitted client available
- **THEN** setup SHALL identify the choice of any one compatible Account Token
  as a prerequisite, not present immediate synchronization as useful work
- **AND** setup, status, check, and sync dry-run SHALL agree on that prerequisite
- **AND** sync dry-run SHALL select no Route or provider by recommendation alone.

## REMOVED Requirements

### Requirement: Explicit Route retirement shares the import transaction

**Reason**: Its title or scenario names spell catalog as the glossary does, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Explicit Route retirement shares the manifest import transaction" in this capability, with every scenario.
