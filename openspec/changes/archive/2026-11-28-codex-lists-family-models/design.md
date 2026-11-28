## Context

`codexCatalogProjection` (`internal/adapter/codex/catalog.go`) reads the installed client's bundled table with
`codex debug models` and passes the selected model alone to `modelcatalog.Document.Project`, which returns the whole
table with that model's entry, or an alias of its family's entry, carrying the declared window. `domain.Runtime` carries
the selected model only. `Config.FamilyModels` already names the models that a client lists for the selected provider
and protocol (the selected model, then the first model of each other family, in configuration order); Claude Desktop's
profile and Claude Code's model map use it. `tools/codex/catalogcheck` proves one model's entry against the real
client.

Codex 0.172.0's source, read on 2026-11-28 at tag `rust-v0.172.0` (commit
`3d86e43b075a`), settles the client side:

- A table named by `model_catalog_json` replaces the bundled one for the process, nothing is fetched or merged, and an
  empty `models` list fails configuration loading (`core/src/config/mod.rs:1105-1107, 2379-2399`;
  `models-manager/src/manager.rs:1061-1084`).
- `visibility` is `list`, `hide` or `none`; only `list` shows an entry in the picker, in `/model <name>`, in the
  app-server's `model/list` and in the models that `spawn_agent` offers (`protocol/src/openai_models.rs:303-312, 1118`).
- Codex reads some entries by slug for its own work: `codex-auto-review` is its reviewer, which becomes the session's
  own model at low effort when the entry is absent (`model-provider/src/provider.rs:99, 571-582`;
  `ext/guardian-reviewer/src/model.rs:32-71`), and its memories ask for `gpt-6-luna` and `gpt-6.1-terra`
  (`provider.rs:106-110`). A configured model without an entry is found by its longest prefix that is a slug, and
  otherwise runs on fallback metadata with a warning on every turn (`manager.rs:1181-1237`;
  `core/src/session/turn_context.rs:1516-1527`).

## Goals / Non-Goals

**Goals:** Codex's picker lists the models that AIGW lists for every other client of the selection; Codex's own work
that reads entries by slug keeps its metadata.

**Non-Goals:** those of the proposal.

## Decisions

### D1. The listed models are the selection's family models that Codex's table describes

- **Rests on:** `Config.FamilyModels` is the one owner of which models a client lists; a model's metadata comes only
  from its own entry or its family's (the accepted rule of `projection-format`).
- **Considered:** (a) every model of the selected provider on the Responses protocol; (b) the team file's
  recommendations for Codex; (c) `FamilyModels`.
- **Chosen because:** (c) is what Claude Desktop and Claude Code already list, so the clients agree and one function
  decides; (a) lists several channels of one family, which the other clients show once; (b) names other providers'
  models, which Codex's single provider block cannot serve.
- **Applies to:** a new `Listed` field of `domain.Runtime`, filled from `FamilyModels` when the selection is resolved,
  each with its model ID, family, name (the label without the provider) and declared window; the Codex projection reads
  it. A listed model with its own entry keeps that entry; one whose family has an entry gets an alias named and windowed
  as the selected model's alias is today, in listing order; one with neither gets no entry.

### D2. Every other entry stays and is hidden

- **Rests on:** a supplied table replaces the bundled one, and Codex reads `codex-auto-review` and its memory models by
  slug; the picker shows only `list` entries.
- **Considered:** (a) remove the entries that are not listed; (b) set their `visibility` to `hide`.
- **Chosen because:** (b) changes what the picker shows and nothing else; (a) would move Codex's reviewer to the
  session's model and its memories to fallback metadata without the operator asking.
- **Applies to:** an entry that Codex lists and that is not a listed model's own entry becomes `hide`; an entry that
  Codex already hides stays as it is, and so does an alias of it, so AIGW never shows what Codex hides. Every other
  field of every bundled entry stays the client's. Display names stay unique across the whole table, hidden entries
  included, under the existing rule.

### D3. The window and the reused copy

The window rule of `projection-format` applies to each listed model's entry, with that model's declared window. When
the client cannot print its table and AIGW reuses its own copy, each listed model's own entry in the copy takes its
window and the copy keeps what it lists: it cannot tell an entry that the client hides from one that AIGW hid, and a
table that the client did not just print gives no new alias. The copy so lags a changed listing until the client prints
its table again. A change of the listing, a model added, removed, moved, renamed or given another window, counts as a
change of Codex's projection where the table that AIGW wrote describes that model.

### D4. The real client proves the listing

`catalogcheck` adds one claim to its run: the effective table that the client renders lists exactly the verified model,
and every entry that the projection hid is hidden there. Its command line does not change.

## Risks / Trade-offs

- A provider whose listed models Codex describes by none gives a table with nothing listed → that is the truthful list
  for that configuration; Codex still loads it, because hidden entries remain, and the selected model runs as it does
  today. Codex 0.172.0 loaded such a table with all 14 entries hidden (2026-11-28); how its terminal picker and Codex
  Desktop show a table that lists nothing was not observed.
- A listed model's own entry takes its declared window, which is AIGW's default of 1,000,000 tokens where the model
  declares none, also where Codex reads that slug for its own work: listing `gpt-6.1-terra` raises the window of the
  model Codex uses for its memories from its bundled 272,000. This is the selected model's existing rule applied to
  every listed model; an operator who wants another window declares it on the model.
- A changed name of a model listed by its own entry counts as a changed projection although the written bytes keep the
  client's name → the table that AIGW wrote cannot tell an alias from an own entry; the cost is one more read of
  Codex's table, never a write.
- An operator who picked a bundled model that the configuration does not hold loses it from the picker → it is not a
  model of the selected provider in AIGW's configuration; `aigw model add` brings it back.
- Codex may read more entries by slug in later versions → they stay in the table, so a lookup keeps its metadata.

## Migration Plan

`aigw sync` rewrites the table that AIGW owns on its next run; no state or schema changes. Rollback is a revert of the
Change and a sync.

## Extension

Entries for models that Codex cannot describe (other vendors' models on the Responses protocol) would join at D1 as a
minimal entry, after a real-client load of one; nothing of it is built here.
