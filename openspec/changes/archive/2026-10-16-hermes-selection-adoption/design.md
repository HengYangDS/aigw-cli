# Design

## Context

AIGW projects every connected Hermes-compatible Route into Hermes'
`config.yaml` as providers and models, selects one of them as `model`, and
records a digest of everything it owns in `config.yaml.aigw-state.json`. Hermes'
own model picker rewrites `model.provider`, `model.default`, `model.base_url`
and `model.api_mode`. That changes the owned digest, so ordinary
synchronization refuses. `aigw use --for hermes` already accepts such a change
through `PrepareAuthorizedSelection`, which proves that only the selection
moved, and then projects the Route that the operator named.

## Goals / Non-Goals

**Goals:** `aigw sync` accepts a native choice inside AIGW's catalogue by
recording the matching Route; no native file is rewritten for that choice; any
other managed edit is still refused.

**Non-Goals:** adoption in `setup`, `use`, `repair` or Route edits; adopting
provider, endpoint or credential command edits.

## Decisions

### The Hermes configuration owner observes the native choice

`hermesconfig.ObserveSelection(path, current)` reads the native file and its
ownership record. It reports no change when the owned digest matches. When it
does not, it applies the selection-only proof and returns the native provider
and model. Any other drift returns the existing refusal. The proof requires
that the provider is AIGW-owned, that `base_url` and `api_mode` equal that
provider's, that the model is in its catalogue, and that replacing the selection
with some owned provider and model reproduces the recorded digest. The proof
searches the recorded providers instead of using the current binding: when a
missing Token deferred the projection after an adoption, the binding already
names the native choice while the record still holds the earlier selection, and
a binding-based proof would refuse forever once the Token returns.

### The Hermes adapter maps the choice to exactly one Route

`client.AdoptHermesSelection` resolves the observed provider through the same
catalogue that names it, giving one Account and protocol, and selects the
catalogue Routes with that Account, protocol and upstream model. Exactly one
Route is adopted through `Config.SetSelectedRoute`. Several Routes are refused,
and the refusal names `aigw use --for hermes --protocol <protocol> <route>` for
each. Nothing is inferred from labels.

### Only `sync` adopts

The sync command calls `Synchronizer.AdoptNativeSelections` before
`DesiredSyncConfiguration`. Running it in shared convergence would let a native
choice override `aigw use --for hermes <route>`, whose named Route must win.

### Projection accepts a native file that already shows the desired choice

After adoption, the desired selection equals the native one, but the ownership
record still holds the previous digest. `hermesPlans` now falls back to
`PrepareAdoptedSelection` when the before configuration resolves. On the same
snapshot that the plan writes from, it accepts the drift only if the
selection-only proof holds and the native selection already equals the desired
one, so this fallback can never overwrite a native choice. When the projection
is semantically unchanged, the plan keeps Hermes' bytes and rewrites only the
ownership record.

### Incomplete self-compensation keeps the reader

When a client adapter's write fails and its own compensation also fails, part
of its configuration may still reference a newly installed credential reader.
`transaction.CompensationFailed` marks such errors, and the client registry
reports them as `ErrProjectionRollbackFailed`, the existing outcome that keeps
the reader. The Hermes, Claude Desktop, Claude Code and Codex adapters use it.

### The result names the adoption

The sync result gains `adopted_selections`, the clients whose Route changed to
match their native choice. The human output shows each adopted Route.

### The CHANGELOG owner keeps each category one list

Prettier accepts a blank line between list items as a loose list, so the
formatter cannot catch a stray gap, and one such gap stayed in `CHANGELOG.md`
since 2026-10-13. `validate-changelog`, which already parses categories and
items, refuses a blank line between items of one category. Other documents keep
Prettier as their only whitespace authority.

## Risks / Trade-offs

- [A Hermes edit that only looks like a selection] → the digest proof covers
  every owned provider field, so a changed endpoint or credential command is
  still refused.
- [Routes sharing an Account, protocol and upstream model] → ambiguity is
  refused with exact commands, never resolved by order.
