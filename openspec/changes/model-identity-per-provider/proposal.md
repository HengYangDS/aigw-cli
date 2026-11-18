## Why

Three findings of 2026-11-18 share one cause: AIGW names and curates models as if one model meant one entry
everywhere, while providers serve the same model under several IDs and at different times.

- Codex Desktop's picker showed "6.2 Sol" twice. AIGW's catalog alias for `gpt-6.2-sol-cdx` copies the family's
  entry with only its slug changed, so it keeps the bundled `gpt-6.2-sol`'s display name.
- AIGW's own derived label is `Provider · Family`, so two models of one provider in one family (`gpt-6.2-sol` and
  `gpt-6.2-sol-cdx`) share a label, and `aigw model add|edit --family` labels a new family with the model's ID (the
  owner's family `gpt-6.2-sol` is labelled `gpt-6.2-sol-cdx`).
- The owner stated first-hand that the line rule is per provider, since providers list new models at different times;
  the file-wide rule would remove a provider's working model when another provider lists a successor first. Suhe.AI,
  a new gateway, also names its models with client prefixes (`claude-code-opus-5-5`, `codex-gpt-6-sol`).

Qwen4-Max (Alibaba, generally available on 2026-11-16) replaces Qwen 3.8 Max 0902 on the providers where it
qualified, under the per-provider rule.

## What Changes

- A model whose ID differs from its family's and begins or ends with the family's ID derives its label with the rest
  of its ID (`DMXAPI · GPT-6.2 Sol · CDX`); two models of one provider with one label are refused.
- A family that `--family` creates is labelled with its own ID.
- The Codex catalog alias carries the model's label without its provider as its display name, and no two listed
  entries of the projected table share one; the catalog verifier compares every field but `slug` and `display_name`.
- The team file's line rule and its test work per provider.
- Qwen4-Max replaces Qwen 3.8 Max 0902 on DMXAPI (Responses) and AIHubMix (Responses and Chat); UCloud keeps Qwen 3.8
  Max 0902 until it lists Qwen4-Max. The Codex and Hermes recommendations are unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `progressive-team-onboarding`: the line rule per provider; derived labels of variants; distinct labels per provider.
- `model-selection`: a created family's label.
- `projection-format`: the Codex alias's display name.

## Impact

`internal/domain` (labels, validation), `internal/cli/model` (created family), `internal/adapter/codex/modelcatalog`
and `internal/adapter/codex` (the alias), `tools/codex/catalogcheck`, `manifests/team.toml`, their tests, the
qualification record and `CHANGELOG.md`. An owner whose configuration holds two models of one provider with one
label must relabel one before AIGW loads it; the owner's configuration holds none (read on 2026-11-18).
