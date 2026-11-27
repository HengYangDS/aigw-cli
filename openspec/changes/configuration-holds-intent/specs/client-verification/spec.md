## MODIFIED Requirements

### Requirement: A real Codex session proves the Codex client depth

At the `client` depth, AIGW SHALL check Codex by running Codex's program, as its declaration resolves, against one
synchronized Codex configuration that AIGW wrote. A direct HTTP request made by AIGW itself MUST NOT be accepted as
proof of the Codex client path.

#### Scenario: A synchronized Codex target is checked

- **WHEN** Codex is checked at the `client` depth with an available
  executable, a synchronized target and a usable provider Token
- **THEN** AIGW SHALL run that executable through its non-persistent execution
  surface with the selected target and model
- **AND** SHALL report the `client` depth only when the client's final
  response is exactly the bounded verification marker.

#### Scenario: Several synchronized Codex targets exist

- **WHEN** Codex has several synchronized targets
- **THEN** AIGW SHALL deterministically select one target for the live request
- **AND** SHALL NOT duplicate the quota-consuming request for equivalent
  targets.

#### Scenario: Codex verification identity is reported

- **WHEN** a Codex session succeeds
- **THEN** AIGW SHALL report the executable version and SHA-256 digest
  measured for that invocation
- **AND** SHALL NOT expose the provider's Token or model response content.

#### Scenario: Codex cannot run the selected model

- **WHEN** the executable is missing or incompatible, its client configuration
  is stale, authentication is unavailable, the request exceeds its bounded
  execution time, or the response marker is absent
- **THEN** the check SHALL fail with an actionable error scoped to that
  boundary
- **AND** a missing declared program SHALL stop the check at the `local` depth, before any request, with a repair
  that names `aigw client edit codex --program <path>`
- **AND** SHALL NOT write client configuration, operator sessions,
  conversation history, model metadata or external gateway state.

#### Scenario: Codex verification reclaims its owned resources

- **WHEN** the Codex client succeeds, fails, exits while its descendants run,
  is interrupted, or reaches its turn deadline
- **THEN** AIGW SHALL stop the client process tree it started and remove its
  verification workspace before returning
- **AND** SHALL leave unrelated processes and protected files unchanged.
