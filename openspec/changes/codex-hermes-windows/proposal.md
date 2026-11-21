## Why

The owner asked on 2026-11-20 that every model run at a context window of 1,000,000 tokens in every client, lowered
only where a real limit is lower, and that AIGW say so. Release 0.9.0 did that for Claude Code and Claude Desktop.
Codex and Hermes still decide the window from their own tables, because AIGW writes none for them:

- Codex 0.171.0 gives the selected GPT model the window of its family's entry: 500,000 or 272,000, of which 95% is
  usable. Its configuration key cannot pass the entry's ceiling of 872,000, so a configured 1,000,000 gave a usable
  828,400 on 2026-11-20.
- Hermes v0.23.3 gives 60 of the operator's 128 models less than 1,000,000: its Codex table answers 272,000 or
  500,000 for 27 GPT model IDs, its catch-all key answers 200,000 for a provider's own Claude Code IDs, and 19
  models that it does not know get 256,000.

## What Changes

- A model may declare `context_window` in `aigw.toml` and in a team file. A model that declares none has
  1,000,000. `aigw model add` and `aigw model edit` set it, and `aigw model list` and `aigw model show` report it.
- Codex: the entry of Codex's model table that describes the selected model carries the declared window as its
  window and its ceiling. For a model whose family the table does not hold, AIGW writes no entry, as before, and
  `aigw check codex` says that Codex's own fallback window applies.
- Hermes: every model of every provider entry that AIGW owns carries the declared window as its own
  `context_length`. The entry's `models` becomes a mapping from model ID, where it was a list of IDs.
- The shipped team file declares a lower window for the families whose vendor documents one.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: a model declares its context window.
- `projection-format`: the entry of Codex's model table that describes the selected model carries the declared
  context window, and the tracked verifier proves it with the real client.
- `client-adapters`: Hermes runs each model at its declared context window.
- `shipped-team-file`: a window below the default is declared with the vendor's number.

## Impact

`internal/domain` (the model's field), `internal/store` (one optional field of schema 8), `internal/cli/model`,
`internal/adapter/codex` and its `modelcatalog`, the tracked verifier `tools/codex/catalogcheck`,
`internal/adapter/hermes`, `manifests/team.toml`, and the guides that name these files. AIGW stays the owner of what it wrote before: Codex's table file and its own provider
entries in Hermes's file. It writes no window into `config.toml`, into Hermes's `model` block or at the level of a
provider entry.

A release before this one does not define `context_window`: it reads a configuration in which no model declares it,
and refuses one in which a model does, as it refuses any field that it does not know. `aigw self rollback` already
refuses to stage a program that cannot read the configuration.

Not in scope: an entry for a Codex model whose family Codex's table lacks (Codex fixes that fallback at 272,000 with
a ceiling of 272,000, and an entry needs the client's own base instructions); entries for models other than the
selected one; Hermes's compression threshold, which is the user's setting; the 1M request header that Hermes does
not send on Anthropic routes; and a paid run near the full window.
