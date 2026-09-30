# Spec Delta

## MODIFIED Requirements

### Requirement: Activation follows present capabilities

Manifest setup SHALL import the reviewed catalogue and project only discovered
clients whose selected Routes have locally available declared authentication.
It SHALL NOT contact a provider or run a native client to decide whether the
declaration can be imported. A connected Token and a native projection are not
claims of accepted authentication, reachable endpoints, or working inference;
`aigw check` owns those live observations. A later synchronization SHALL
rediscover and adopt a newly installed admitted client without requiring
manifest re-import.

#### Scenario: Only Claude Code is installed

- **WHEN** a connected Account has both Anthropic and Responses routes but
  only Claude Code is discovered
- **THEN** setup SHALL configure only the Claude route
- **AND** an unavailable Responses endpoint SHALL NOT block the import.

#### Scenario: Provider is offline during manifest import

- **WHEN** one compatible Account Token is available and its provider cannot
  be reached
- **THEN** manifest setup SHALL retain the catalogue, selection, and local
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
compatible variable as an equal choice and state that one is sufficient. An
environment Token MAY activate an unselected reviewed recommendation through
`aigw sync`. With a writable backend, Token rotation SHALL preserve Route and
Client Binding selection and name the required explicit `aigw use --for`
continuation. A newly installed selected client is activated with `aigw sync`;
`aigw check` only verifies enabled Routes. Manifest setup MUST support `--json`,
share one semantic result, and never expose credentials.

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
- **AND** SHALL NOT imply that every catalogue Account Token is required.

#### Scenario: A writable Account Token arrives after import

- **GIVEN** a team catalogue with no selected Client Binding
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

- **WHEN** a team catalogue was imported without a connected Account
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
- **THEN** setup reports the catalogue as imported
- **AND** does not claim any endpoint, credential, or client is ready.

#### Scenario: Import-only setup has no actionable synchronization

- **WHEN** a valid team catalogue is imported with no compatible Token and no
  admitted client available
- **THEN** setup SHALL identify the choice of any one compatible Account Token
  as a prerequisite, not present immediate synchronization as useful work
- **AND** setup, status, check, and sync dry-run SHALL agree on that prerequisite
- **AND** sync dry-run SHALL select no Route or provider by recommendation alone.

## ADDED Requirements

### Requirement: Explicit Route retirement shares the import transaction

Configuration import SHALL retain local-only Routes unless the operator names
each one with `--retire-route`. A named Route SHALL exist locally, be absent
from the incoming manifest, and not be selected by any Client Binding. The
result SHALL have no recommendation referencing a retired Route. A Model
referenced by a retired Route SHALL be removed only if no remaining Route references it
and the incoming manifest does not declare it. Account metadata and Tokens
SHALL remain untouched by retirement. Import, retirement, and any affected
native client projection SHALL use one guarded commit with compensation.

#### Scenario: Reviewed catalogue replaces obsolete recommendations

- **WHEN** one import names multiple unselected local Routes for retirement
- **AND** the incoming manifest replaces recommendations that referenced them
- **THEN** the Routes and only their unreferenced, undeclared Models SHALL be
  removed in the same commit as the new public catalogue
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
