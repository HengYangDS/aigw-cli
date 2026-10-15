# Hermes Selection Adoption

## Why

Hermes lets the operator pick another model from the catalogue that AIGW
projects. AIGW 0.4.1 treats that choice as an external edit: on 2026-10-15,
after the operator chose DMXAPI Claude Opus 5.5 as Hermes' default, every
`aigw sync` failed with "managed Hermes settings changed outside AIGW; preserve
the edit before synchronization" and recommended only `aigw check`. Only an
explicit `aigw use --for hermes` could recover, and the error did not name it.
The product contract already says that client-owned model choices remain
client-owned.

## What Changes

- `aigw sync` adopts a Hermes model choice that stays inside AIGW's projected
  catalogue: it records the one matching Route as the Hermes Client Binding,
  reports the adoption and leaves the native selection unchanged.
- When the native choice matches more than one Route, synchronization refuses
  and names the `aigw use --for hermes` command for each candidate.
- Any other change to AIGW-owned Hermes settings is still refused.
- `aigw use --for hermes` keeps replacing the native selection with the Route
  that the operator names.

## Capabilities

`product-control-plane` gains the requirement "Synchronization adopts a Hermes
model choice".

## Impact

The owners are `internal/hermes/configuration`, which proves selection-only
drift and reads the native choice, and the Hermes adapter in `internal/client`,
which maps that choice to a Route. Synchronization commits the adopted binding
through its existing guarded transaction. Other clients, credentials and Hermes
sessions are unchanged. No model, protocol or Route is inferred from a name:
adoption requires the exact provider, protocol and upstream model that one
admitted Route projects.

Non-goals: adopting edits to provider entries, endpoints or credential
commands; adoption by commands other than `sync`; changing Codex or Claude
selection handling.
