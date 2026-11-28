## Why

On 2026-11-25 the operator saw older GPT models in Codex's model picker beside GPT-6.2 and said that after importing the
team file only models that follow its curation should remain. On 2026-11-28 the table that AIGW projects for Codex
lists twelve models (read from the projected file): the eleven that Codex 0.172.0 ships as visible, among them
`gpt-6-astra`, `gpt-6.1-sol`, `gpt-6-sol`, the three `gpt-5.6` models and `gpt-5.5`, and AIGW's alias of the selected
model. AIGW copies the client's whole bundled table, as `projection-format` requires ("the complete bundled table plus an
alias"), so every model that Codex ships is listed whatever the configuration holds, while Claude Desktop and Claude
Code already list the models that AIGW's configuration names.

## What Changes

- **BREAKING** Codex's projected table lists the models that a client lists for the selected provider and protocol, the
  selected model and the first model of each other family in configuration order, wherever Codex's own table describes
  one by its own entry or by its family's. Each listed model carries its declared context window.
- Every other entry of Codex's table stays in the projected table and is hidden from the picker. Codex reads entries by
  slug for its own work, such as its auto-review model and its memories, and a supplied table replaces the bundled one,
  so removing them would change that work.
- A model that Codex's table describes by neither entry is still listed by none, as today.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `client-adapters`: a new requirement names the models that Codex's table lists, beside the existing rules for
  Claude Desktop and Claude Code.
- `projection-format`: the Codex table requirements stop requiring the complete bundled table unchanged; the declared
  window applies to every listed model.

## Impact

`internal/adapter/codex` and `internal/adapter/codex/modelcatalog` (the projected table), the selection's runtime data
(the listed models), `tools/codex/catalogcheck` (the real-client verification), their tests, `CHANGELOG.md` and the two
accepted specifications above. Reuse: `domain.Config.FamilyModels` already owns which models a client lists; Codex's
`visibility` field owns what its picker shows. Boundary: entries that Codex marks hidden, every field of a bundled entry
other than `visibility`, and the operator's own `model_catalog_json` stay untouched. Non-goals: entries for models that
Codex cannot describe, and other providers' models in Codex's picker.
