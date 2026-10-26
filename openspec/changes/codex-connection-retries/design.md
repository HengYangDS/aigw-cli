# Design

## Context

See [the proposal](proposal.md) for motivation. Client Binding owns explicit
native options; Runtime resolves them for projection. Codex's provider reader
and canonical rendering define its managed-block hash. The forwarding sidecar
exists for predecessor-readable destinations and requires an endpoint; it is
not a generic policy carrier.

## Goals / Non-Goals

**Goals:** one explicit connection policy from declaration to guarded projection
and recovery, with distinguishing regression evidence.

**Non-Goals:** request execution or idempotency, automatic endpoint discovery,
provider-name or URL inference, unsupported client adapters, release or install.

## Decisions

### Native budgets remain optional Client Binding declarations

An optional `retry_policy` table carries independently optional nonnegative
`request_max_retries` and `stream_max_retries`. Zero is an explicit value, not
absence. The Codex adapter admits these native fields; other adapters refuse
the declaration. Undeclared fields stay absent, preserving the client's defaults.
No policy is attached to Accounts, Routes, Models or a global runtime setting.

This mirrors the documented native contract and reuses the already qualified
Codex consumer, rather than inventing a vendor policy name or disabling retries
globally. The Store's existing optional-component transaction records policy in
`.retries.toml`, bound to connection identity. Canonical configuration bytes stay
readable by immutable cached credential readers, which do not consume transport
policy. Snapshots, backups, compensation and comparisons include the component;
verification checkpoints retain the policy they actually verified. A universal
retry-owner abstraction would promise semantics the other clients do not expose.

### Selection changes one connection deliberately

Use a meaningful connection-change value for destination and retry intent,
replacing the existing forwarding-only selection argument. CLI budget flags
merge only supplied fields; `--native-retries` explicitly removes the policy.
Changing the upstream identity or effective destination with an existing policy
requires explicit retry intent. The shared transaction checks Account edits and
setup as well as selection; Account creation admits the connection before Token
acquisition. Model-only changes preserve it. Runtime overrides
for a different connection do not inherit the selected connection's policy.

### Native source acceptance repairs its diagnosed owners

The user authorized the three gate repairs in this same lane. Mise startup
ignores the external configuration tier through native process inputs while
retaining project discovery and task working directories; one CUE
contract projects both Forge environments. GoReleaser receives invocation-local
Git selection for an existing peer without changing remotes or persisted Git
configuration. Mirror fixtures preserve both complete-mirror and asset-only
absence, including the native metadata warning when the whole mirror is absent.
Neither warnings nor missing checksums are suppressed to obtain a green gate.

### Projection and compensation retain their existing owners

The existing provider struct parses and renders both optional fields. The same
rendered block feeds ownership hashes, validation, synchronization and rollback.
An external budget edit remains a conflict; parsing support does not authorize
overwriting it. A typed provider conflict survives diagnostic rendering instead
of becoming a misleading line/column error from a reconstructed fragment.

### Program rollback proves reader capability, not a version fence

The Store derives an isolated admission document with selected policy inline;
it does not rewrite persistent configuration. The existing retained program's
native `config export` probe must read this complete declaration, so an old
strict reader cannot silently ignore the optional component and pass. With no
retained reader supplied, Homebrew rollback preparation refuses before credential
custody rather than assert unproved capability. A refusal preserves program,
configuration and policy;
recovery names a compatible program or native configuration rollback, not manual
field deletion. An unsuccessful reader check reports its observed refusal without
inferring configuration incompatibility from a process exit alone. No arbitrary
AIGW or Codex version threshold is introduced.

## Risks / Trade-offs

- Native field support differs between clients → admit only the adapter with
  documented and actual-consumer evidence; do not emulate other adapters.
- Old readers cannot accept a declared policy → preserve a truthful, reversible
  refusal rather than silently enabling retries during downgrade.
- Optional nested values could alias during preparation → clone policy values
  with Config and resolve immutable Runtime inputs.
- Zero budgets reduce transparent client recovery → explain that the endpoint
  owns recovery; this does not promise exactly-once inference.

## Migration Plan

Existing configurations need no migration or rewrite. Operators opt in through
configuration or explicit `use` budget flags. The native configuration backup
and projection receipt restore prior policy, fields and presence. Claude will
qualify the changed release and install after source acceptance; no live effect
is authorized in this lane.
