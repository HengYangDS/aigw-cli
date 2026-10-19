# Design

## Context

AIGW projects the selected Codex Route as a root `model_provider`, a root
`model`, a managed provider table, an optional model catalogue and scheduler
values, and records what it wrote in a sidecar. Codex's model picker rewrites
the root `model`. Ordinary synchronization treats any root change as a
conflict, while `aigw use --for codex <route>` may replace root values after
the rest of the projection still matches its sidecar. Hermes already adopts a
native model choice during synchronization through `AdoptNativeSelections`,
which this design follows.

## Goals / Non-Goals

**Goals:** `aigw sync` records an unambiguous Codex model choice as the Codex
Client Binding, refuses anything else with the existing protection, and turns
an ambiguous or unknown choice into actionable selection commands.

**Non-Goals:** adopting provider changes, adopting in repair or inspection,
editing Codex conversations, and changing other clients' selection rules.

## Decisions

### Adoption follows the Hermes owner contract

`Synchronizer.AdoptNativeSelections` asks the Codex adapter for its adoption,
as it already asks Hermes. Only synchronization calls it, so repair, status and
verification keep reporting drift, and the authorized transition of
`aigw use` still replaces the native choice with the named Route.

### The sidecar proves a model-only change

The Codex package observes a target and reports a chosen model only when the
sidecar's provider table hash, provider selection, scheduler ownership and
catalogue bytes still match and the root model differs from the recorded
projected model. A comment-only rewrite is not a change. Any other drift is
not adopted; the ordinary plan then refuses with its existing message, so no
second refusal path appears.

### A Route matches within the selected Account

The managed provider table already sends Codex requests to the selected
Account's endpoint with its credential command. A matching Route therefore has
the selected Account, a Codex-compatible protocol equal to the selected one,
and an upstream model equal to the chosen value. One match is adopted; none or
several fail with `aigw use --for codex` commands. All targets of the binding
that changed must name the same model; unchanged targets follow the binding.

### Projection keeps the chosen value

After adoption the desired root model equals the value Codex wrote. The
transition accepts a root model that already holds the value it is about to
write, because the write cannot lose an edit; every other ownership check is
unchanged. The provider table name and the catalogue follow the adopted Route.

## Risks / Trade-offs

- [The chosen model belongs to no Route of the Account] → synchronization
  fails with the selection command and leaves the native choice in place.
- [A sidecar from before projected models were recorded] → the change cannot
  be attributed and is not adopted; `aigw use` remains the path.
- [A model with several Routes, such as an admitted and a deprecated copy] →
  the operator chooses with the listed commands.

## Migration Plan

No data migration. After archive, the next AIGW release carries the behavior
through the existing release workflow; acceptance then changes the root model
of an isolated Codex home the way Codex writes it, runs `aigw sync`, and
repeats an unchanged dry run.

## Open Questions

- None.
