## MODIFIED Requirements

### Requirement: Independently admitted native clients

Codex and Claude Code SHALL be independent Adapters owning discovery, projection, authentication, rollback,
verification, status, and withdrawal of AIGW state. Provider-Token credential commands MUST use an absolute qualified
reader, never store Tokens in client files, and never rely on a package-manager CLI link. A client's own sign-in and
preferences SHALL remain client-owned. A new client MUST add an Adapter without changing provider policy or other
Adapters.

#### Scenario: Verification uses synchronized settings

- **WHEN** AIGW verifies an admitted client
- **THEN** it SHALL use the client's synchronized settings without a wrapper.

#### Scenario: One admitted client is absent

- **WHEN** setup discovers only Codex or only Claude Code
- **THEN** AIGW SHALL configure only the present client
- **AND** it SHALL explicitly leave the absent client untouched

#### Scenario: Claude launches outside the installer shell

- **WHEN** Claude Code requests a credential from an enabled AIGW projection
- **THEN** `apiKeyHelper` SHALL invoke the selected qualified reader
- **AND** credential retrieval SHALL not depend on the caller's PATH
- **AND** the projected settings SHALL contain no plaintext Token

#### Scenario: Codex authenticates an explicit native provider

- **WHEN** a version 9 configuration or team file names, in `[clients.codex]` or a Codex recommendation, an explicit
  native provider through `model_provider` or the client's own authentication through `authentication`
- **THEN** loading the file SHALL refuse that field, which schema 9 does not define
- **AND** Codex SHALL authenticate only through AIGW's own provider, whose credential command invokes the selected
  qualified reader with the fingerprint of Codex's selected provider and endpoint
- **AND** no plaintext Token SHALL be written to public configuration

#### Scenario: Claude uses an Anthropic-compatible provider

- **WHEN** explicit verification invokes Claude Code for a selected model
- **THEN** the native client SHALL consume its synchronized settings in bare,
  nonpersistent mode
- **AND** stale AIGW-owned Anthropic environment overrides SHALL be removed
- **AND** unrelated client preferences, including beta controls, SHALL remain
  unchanged rather than becoming a new AIGW authority

#### Scenario: The installed executable path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the AIGW executable path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: The credential reader path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the selected reader path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: Credential retrieval is not admitted

- **WHEN** a credential request names an unsupported client, a disabled
  client, a fingerprint that matches no provider endpoint, or a provider
  without a Token
- **THEN** the request SHALL fail without writing credential bytes to standard
  output

#### Scenario: A future agent is admitted

- **WHEN** Hermes or another agent supporting third-party LLM APIs is proposed
- **THEN** admission SHALL require only that agent's adapter, declaration, and
  fixtures and SHALL NOT change provider policy, external-gateway behavior,
  command roots, or an existing adapter

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW SHALL project the selected model once into that shared home
- **AND** SHALL NOT create a second Desktop-specific configuration authority

#### Scenario: An adapter is proposed for another agent client

- **WHEN** a new agent client such as pi or OpenCode is proposed
- **THEN** its admission SHALL independently prove executable discovery,
  protocol/model selection, credential ownership, native configuration merge,
  rollback, withdrawal and a real tool-loop journey
- **AND** AIGW SHALL NOT advertise support based only on a writable config
  path or a successful endpoint probe.

### Requirement: Composable extension boundary

AIGW SHALL keep configuration and client projection separate from API traffic. Endpoints and models SHALL enter as
provider data. New clients SHALL require complete Adapters, unsupported credential contracts SHALL require admission,
and incompatible wire behavior SHALL remain in a separate data plane. Gateways SHALL remain optional endpoints.
Dependencies MAY be admitted only when total owned complexity falls.

#### Scenario: Add a compatible Provider endpoint

- **WHEN** an endpoint satisfies an admitted client protocol and the existing
  provider authentication contract
- **THEN** an operator SHALL add it through configuration data
- **AND** AIGW SHALL NOT add provider-name branching or a provider-specific
  runtime package.

#### Scenario: Add a client integration

- **WHEN** a new local client requires AIGW-managed configuration
- **THEN** it SHALL be admitted through its own discovery, planning, guarded
  projection, verification, rollback, and uninstall boundary
- **AND** it SHALL NOT reuse another client's conditional path or private state.

#### Scenario: A client adapter is registered

- **WHEN** an adapter is registered, including a test-only rehearsal adapter
- **THEN** it SHALL pass one shared adapter contract: its plan lists every
  file that applying it may change and states exactly whether applying it
  changes one, rollback restores the prior bytes and modes, withdrawal leaves
  no owned file, and inspection, verification and planning change no file and
  leave the caller's configuration unchanged
- **AND** an adapter that fails any part of that contract SHALL NOT be admitted.

#### Scenario: Compose with a traffic gateway

- **WHEN** an operator selects a general gateway or narrow compatibility
  service
- **THEN** AIGW SHALL model its URL as an ordinary provider endpoint
- **AND** SHALL NOT install, supervise, configure, embed, or copy the service's
  traffic policy.

#### Scenario: Evaluate a mature dependency

- **WHEN** a library or framework is proposed for an AIGW-owned boundary
- **THEN** admission SHALL demonstrate a net reduction in owned complexity
- **AND** popularity or feature count alone SHALL NOT justify adoption.

### Requirement: Extension preserves the control-plane core

An ordinary provider SHALL be added through provider, endpoint, model, family,
and optional balance-probe declarations. A new client SHALL be added through
one admitted client Adapter and its conformance fixtures. Neither extension
SHALL introduce provider-name branching in the core, reuse another client's
projection, or make an optional product a dependency.

#### Scenario: Add a client-native cloud model service

- **WHEN** an AWS model service exposes an admitted client protocol but needs the client's own credential chain and
  request signing
- **THEN** AIGW SHALL offer no configuration that hands the client's authentication to that chain, since schema 9
  defines no `authentication` key in a client or a recommendation
- **AND** AIGW neither stores the cloud credential nor implements a signing
  Adapter.

#### Scenario: Add another agent client

- **WHEN** Hermes, OpenCode, Pi, Qoder, or another client is admitted
- **THEN** it supplies its own discovery, projection, credential, rollback,
  verification, and uninstall contract
- **AND** existing provider data and client Adapters remain unchanged.

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

## REMOVED Requirements

### Requirement: Client-scoped Codex native provider

**Reason**: Schema 9 removes `model_provider` and `authentication`, so Codex always receives AIGW's own `aigw`
provider table with the credential command.

**Migration**: Delete both keys from `aigw.toml` and team files; "Credential commands match a provider endpoint" in
`projection-format` owns Codex's one provider table.

### Requirement: A configured client program is never replaced

**Reason**: It moves to the new capability `client-programs`, which owns which program a client uses.

**Migration**: `client-programs` holds it under the same title with its schema 9 text and every scenario.

### Requirement: A missing program stops its client

**Reason**: It moves to the new capability `client-programs`, which owns which program a client uses.

**Migration**: `client-programs` holds it under the same title with its schema 9 text and every scenario.
