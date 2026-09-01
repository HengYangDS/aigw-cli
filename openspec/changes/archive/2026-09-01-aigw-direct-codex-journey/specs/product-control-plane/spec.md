## MODIFIED Requirements

### Requirement: Independent product composition

AIGW SHALL treat every valid Account endpoint as an endpoint choice, whether it
is a direct provider HTTPS endpoint or an independently operated gateway such
as Codex Responses Proxy. AIGW MUST NOT import, invoke, install, configure,
diagnose, reload, uninstall, or roll back that gateway, and the gateway MUST
NOT acquire AIGW state.

#### Scenario: An Account selects direct HTTPS

- **WHEN** an operator selects a valid direct HTTPS Account endpoint
- **THEN** AIGW SHALL project that endpoint without requiring a local gateway
- **AND** verification SHALL exercise the selected client through that direct
  endpoint

#### Scenario: An Account selects loopback HTTP

- **WHEN** an operator selects a valid loopback endpoint
- **THEN** AIGW SHALL treat it only as an external endpoint without product,
  fixed-port, path, or lifecycle assumptions

#### Scenario: Governed Codex deployment uses the Proxy

- **WHEN** a governed deployment selects gateway endpoints for one or more
  Codex Profiles
- **THEN** AIGW SHALL project those endpoints by the same Account and Route
  semantics used for direct HTTPS endpoints
- **AND** runtime evidence SHALL verify each selected client path without giving
  AIGW gateway lifecycle ownership or encoding a fixed product, path, or port

#### Scenario: An external gateway is unavailable

- **WHEN** an independently operated gateway is absent or unhealthy
- **THEN** AIGW SHALL report only the selected endpoint's observed failure
- **AND** SHALL NOT install, start, stop, repair, or reconfigure the gateway

## ADDED Requirements

### Requirement: Real Codex client route verification

AIGW SHALL verify a selected Codex Profile by running the configured and
admitted Codex executable against one synchronized AIGW projection. A direct
HTTP request made by AIGW itself MUST NOT be accepted as proof of the Codex
client path.

#### Scenario: A synchronized Codex target is verified

- **WHEN** an operator verifies a Codex Profile with an available executable,
  synchronized target, and usable Account Token
- **THEN** AIGW SHALL run that executable through its non-persistent execution
  surface using the selected target and Profile model
- **AND** SHALL accept the verification only when the client's final response is
  exactly the bounded verification marker

#### Scenario: Several synchronized Codex targets exist

- **WHEN** an operator verifies a Codex Profile whose adapter owns several
  synchronized targets
- **THEN** AIGW SHALL deterministically select one target for the live request
- **AND** SHALL NOT duplicate the quota-consuming request for equivalent targets

#### Scenario: Codex verification identity is reported

- **WHEN** a Codex verification succeeds
- **THEN** AIGW SHALL report the executable version and SHA-256 digest measured
  for that invocation
- **AND** SHALL NOT expose the Account Token or model response content

#### Scenario: Codex cannot execute the selected route

- **WHEN** the executable is missing or incompatible, its projection is stale,
  authentication is unavailable, the request exceeds its bounded execution
  time, or the response marker is absent
- **THEN** verification SHALL fail with an actionable error scoped to that
  boundary
- **AND** SHALL NOT write client configuration, operator sessions, conversation
  history, model metadata, or external gateway state
