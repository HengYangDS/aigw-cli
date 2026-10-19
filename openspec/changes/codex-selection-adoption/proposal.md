# Codex Selection Adoption

## Why

Codex writes the operator's model choice to the root `model` of its own
configuration, the same value AIGW projects for the selected Route. When the
operator picks another model in Codex, `aigw sync` refuses with "AIGW-managed
model selection changed; refusing to overwrite user edits", although that
model may be exactly another Route of the same Account. The operator then has
to know that `aigw use --for codex <route>` must come first. Hermes already
records such a native choice during synchronization.

## What Changes

- `aigw sync` adopts a Codex model choice. When the root model is the only
  value that AIGW owns in a Codex target and that changed, and exactly one
  Route of the selected Account and protocol has that upstream model,
  synchronization records that Route as the Codex Client Binding, projects it
  without rewriting the chosen model, and reports the adopted Route.
- When no Route or several Routes match, synchronization changes nothing and
  names `aigw use --for codex` with each matching Route, or the selection
  command when none matches.
- Any other change to an AIGW-owned Codex value, such as the provider, its
  table, the scheduler or the catalogue, still refuses as before.
- Inspection and repair keep reporting the change; an explicit
  `aigw use --for codex <route>` still wins over the native choice.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: gains the requirement "Synchronization adopts a
  Codex model choice".
- `projection-format`: "Codex selections respect TOML scope and literal
  identity" lets synchronization adopt a model-only root change.

## Impact

Only synchronization adopts, and only a model-only change; the Codex provider,
endpoint, credential command, scheduler, catalogue, named profiles and every
other user setting stay guarded. Codex conversations keep the models they
recorded. Nothing is removed or renamed.

Out of scope: adopting a provider change, adopting in `aigw repair`, Claude
Code's model preference, which AIGW already treats as client-owned, and
credential-reader continuity.
