# Spec Delta

## MODIFIED Requirements

### Requirement: Real Codex client route verification

AIGW SHALL verify a selected Codex Route by running the configured and
admitted Codex executable against one synchronized AIGW projection. A direct
HTTP request made by AIGW itself MUST NOT be accepted as proof of the Codex
client path.

#### Scenario: A synchronized Codex target is verified

- **WHEN** an operator verifies a Codex Route with an available executable,
  synchronized target, and usable Account Token
- **THEN** AIGW SHALL run that executable through its non-persistent execution
  surface using the selected target and Route model
- **AND** SHALL accept the verification only when the client's final response is
  exactly the bounded verification marker

#### Scenario: Several synchronized Codex targets exist

- **WHEN** an operator verifies a Codex Route whose adapter owns several
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

#### Scenario: Codex verification reclaims its owned resources

- **WHEN** the Codex client succeeds, fails, exits while its descendants run, is
  interrupted, or reaches its turn deadline
- **THEN** AIGW SHALL stop the client process tree it started and remove its
  verification workspace before returning
- **AND** SHALL leave unrelated processes and protected files unchanged
