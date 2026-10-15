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
does not, it applies the existing selection-only proof against the current
binding's selection and returns the native provider and model. Any other drift
returns the existing refusal. The proof is unchanged: the provider must be
AIGW-owned, `base_url` and `api_mode` must equal that provider's, the model
must be in its catalogue, and replacing the selection with the recorded one must
reproduce the recorded digest.

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
`PrepareAdoptedSelection` when the before configuration resolves. It accepts the
drift only if the selection-only proof holds and the native selection already
equals the desired one, so this fallback can never overwrite a native choice.
When the projection is semantically unchanged, the plan keeps Hermes' bytes and
rewrites only the ownership record.

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
