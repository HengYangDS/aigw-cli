# client-verification Specification

## Purpose

Define how a real client session proves a model at the client depth of `aigw check`: what a Hermes
verification owns, reuses and releases, what a Codex session must show, and which lines of a client count
as a warning.

## Requirements

### Requirement: Hermes verification excludes unrelated update services

When `aigw check` runs Hermes at the `client` depth, it SHALL invoke the native
client with Hermes's selected model without requiring the client's upstream
software-update service. The disposable verification environment SHALL
preserve the operator's Hermes configuration and credentials, and version-probe
failures SHALL be classified without exposing raw vendor output or private
paths.

#### Scenario: Vendor update service is unavailable

- **WHEN** the selected inference endpoint is healthy but the Hermes software
  update service is unavailable
- **THEN** AIGW can observe the native client version and complete one bounded
  request for the selected model without contacting the update service
- **AND** no user's Hermes configuration or credential is changed.

#### Scenario: Hermes version probe fails

- **WHEN** the native Hermes version probe times out or exits unsuccessfully
- **THEN** verification fails with the corresponding cause category
- **AND** it does not report successful inference, retry the probe, prompt for
  credentials, or disclose raw stderr, Token material, or private paths.

### Requirement: Hermes verification reuses installed dependency state

When the configured Hermes home's data root holds the installation's committed
dependency environment and managed tools, AIGW's disposable Hermes verification
environment SHALL reuse them without borrowing mutable home-maintenance records,
and its own boot records SHALL remain disposable. The probe SHALL NOT activate
operator plugins or credential-backed memory providers, or install, synchronize,
rebuild, or republish the operator's dependencies, tools, launchers, or boot
records.

#### Scenario: Verification keeps the native dependency selection

- **WHEN** the disposable environment reuses the committed dependencies
- **THEN** native dependency selection SHALL retain its original physical lock
  and generation leases
- **AND** SHALL include the complete native live-profile dependency-member
  union with its original resolved identities.

#### Scenario: Hermes keeps installation state under its data root

- **WHEN** the installed Hermes resolves its committed dependency environment
  and managed tools from the data root of the configured Hermes home
- **THEN** AIGW observes the native client version and completes one bounded
  request from a disposable home outside the operator's native data roots
- **AND** verification refuses on-demand installation, so the installed
  dependencies, tools, and launchers are not synchronized or republished
- **AND** removing the disposable home leaves the operator's Hermes
  configuration, installation state, and launchers unchanged.

#### Scenario: Installation state cannot be shared

- **WHEN** the host refuses to expose the configured home's installation state
  to the disposable home
- **THEN** verification fails before starting Hermes
- **AND** it does not provision another dependency environment, retry, or
  disclose private paths.

#### Scenario: Installed dependencies include native plugin members

- **WHEN** default or live named profiles contribute plugin or memory-provider
  members to the committed dependency environment
- **THEN** disposable selector-only profiles retain their native names,
  disabled selections and resolved source identities without copying credentials
- **AND** retired profiles, invalid profile names and the default alias do not
  change the dependency union
- **AND** the inference probe disables plugin and memory-provider activation
  while native dependency and boot-record checks remain effective.

#### Scenario: Native launch effects cannot be isolated

- **WHEN** the installed platform's boot path cannot preserve the operator's
  launchers or safely borrow its committed dependency selection
- **THEN** AIGW refuses verification before starting the client
- **AND** it does not invent a private selection lock, provision dependencies,
  modify client configuration, or claim readiness
- **AND** the refusal discloses no credential diagnostics.

#### Scenario: Required installation state is unavailable

- **WHEN** the installed Hermes cannot resolve required dependency state from
  the configured native root
- **THEN** verification reports the native failure without claiming readiness
- **AND** it does not provision another dependency environment, retry, or
  disclose private paths.

### Requirement: Hermes verification releases its gateway before deleting its home

Hermes verification SHALL bound the idle lifetime of the gateway its chat
starts, keep that gateway's host locks inside the verification home, and
delete the home only after observing that no process holds its gateway lock.
When the lock stays held after one native stop request, verification SHALL
retain the home, name it, and report incomplete verification.

#### Scenario: The verification gateway exits after the chat

- **WHEN** the verification chat returns and its gateway exits on idle
- **THEN** verification SHALL delete its home only after the gateway lock is
  free
- **AND** the operator's host lock directory SHALL remain untouched.

#### Scenario: The verification gateway outlives its idle bound

- **WHEN** the gateway lock is still held after the idle bound
- **THEN** verification SHALL request `hermes gateway stop` once with closed
  input for that home and wait again
- **AND** it SHALL send no signal to the gateway.

#### Scenario: The verification gateway keeps running

- **WHEN** the gateway lock is still held after the stop request
- **THEN** verification SHALL retain the home and name it in its error
- **AND** it SHALL NOT report completed verification.

### Requirement: Hermes verification waits for its gateway process to exit

Hermes verification SHALL delete its disposable home only after every gateway
that Hermes's own records place in it has ended: the free gateway lock, the
process that `gateway_state.json` records, and a launched gateway's lifecycle
record. A gateway that outlives the bound SHALL leave the home in place and
named in the error.

#### Scenario: The gateway releases its lock before it exits

- **WHEN** the gateway lock is free but the recorded gateway process still runs
- **THEN** verification SHALL wait for that process to exit before it deletes
  the home
- **AND** verification SHALL keep the home and name it in the error when the
  process outlives the bound.

#### Scenario: No gateway recorded a run

- **WHEN** the home has no gateway state file and no gateway launch log
- **THEN** the free gateway lock alone SHALL decide whether the home is deleted.

#### Scenario: A launched gateway is still starting

- **WHEN** verification ends while a gateway launched into its home has written
  its launch log but has not yet locked or recorded itself
- **THEN** verification SHALL keep the home until the gateway's lifecycle record
  shows it exited, or names a process that is gone
- **AND** if the bound passes first, verification SHALL retain the home and name
  it in an error that does not claim the gateway still runs.

### Requirement: A real Codex session proves the Codex client depth

At the `client` depth, AIGW SHALL check Codex by running its configured Codex
executable against one synchronized Codex configuration that AIGW wrote. A
direct HTTP request made by AIGW itself MUST NOT be accepted as proof of the
Codex client path.

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
- **AND** a missing configured executable SHALL stop the check at the `local`
  depth, before any request, with the repair
  `aigw client edit codex --executable <path>`
- **AND** SHALL NOT write client configuration, operator sessions,
  conversation history, model metadata or external gateway state.

#### Scenario: Codex verification reclaims its owned resources

- **WHEN** the Codex client succeeds, fails, exits while its descendants run,
  is interrupted, or reaches its turn deadline
- **THEN** AIGW SHALL stop the client process tree it started and remove its
  verification workspace before returning
- **AND** SHALL leave unrelated processes and protected files unchanged.

### Requirement: Native warning symbols are diagnostics

A native diagnostic line whose first visible character is an explicit warning
or error symbol, `⚠`, `✗`, `✖`, `✘` or `❌`, SHALL count as a warning in
every check that rejects explicit native warnings. Check marks, progress
glyphs and symbols inside a line SHALL NOT.

#### Scenario: Hermes reports an out-of-sync installation

- **WHEN** Hermes exits zero after printing a line that starts with `⚠`
- **THEN** verification SHALL report incomplete qualification without the
  private diagnostic.

#### Scenario: A client prints ordinary progress

- **WHEN** a client prints `session_id:`, a `✓` result or a `↻` resume line
- **THEN** that line SHALL NOT be a warning.
