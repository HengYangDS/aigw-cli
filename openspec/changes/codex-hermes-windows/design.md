## Context

The proposal states the need and the scope; the delta specifications state the behavior. This file records why the
behavior has this shape. Its facts were read on 2026-11-21 and 11-22 in Codex's source at tag `rust-v0.171.0`, in
Hermes v0.23.3, in OpenAI's model pages and in each vendor's own documentation; the working evidence with every
source is kept outside the repository, and each decision below names the measurement or test that it rests on.

## Goals / Non-Goals

**Goals:** one declaration of a model's window; Codex and Hermes use it where the client lets them; what a client
caps is said by AIGW, not discovered by the operator.

**Non-Goals:** a window for a Codex model that Codex's table does not describe; entries for models other than the
selected one; any setting of a client that is the user's own (Hermes's compression threshold); a paid run near the
full window.

## Decisions

### A window is a field of the model, with one default

- **Rests on:** the stored configuration and the team file describe a model by provider, ID, family, label,
  protocols and capabilities, and no adapter holds a window today. The limit that matters is the one of the model as
  its provider serves it: two providers may serve one family with different limits.
- **Considered:** a number per client in each adapter; a table of families inside the program; a field of
  `[[families]]`.
- **Chosen because:** a model is the unit that the commands address and that a provider serves, so its window is
  declared there, once, and every adapter reads it. The default of 1,000,000 is one named constant of the domain. A
  table inside the program could not be corrected without a release, and a family's field could not say that one
  provider's channel stops lower.
- **Applies to:** the model in `internal/domain`, the resolved runtime, the store's schema 8 document and the team
  file, `aigw model add`, `edit`, `list` and `show`.

### The field is optional in schema 8 and is written only when declared

- **Rests on:** the store refuses a field that its schema does not define (`decodeDocument`), and no field has been
  added to `aigw.toml` since 0.7.0. `aigw self rollback` refuses to stage a retained program that cannot read the
  configuration, and the release's own acceptance holds that (`TestNativeRollbackConfigurationAdmission`).
- **Considered:** schema 9, which every earlier release refuses whatever the file holds; a reader of schema 8
  inside a schema 9 program, which is conversion code.
- **Chosen because:** a model without a declaration writes nothing, so a configuration in which no model declares a
  window is byte for byte what the previous release writes and reads. Only a configuration that uses the field is
  refused by a release that does not define it, by the field's name and line, and the rollback admission already
  stops a program rollback over such a configuration with the configuration untouched.
- **Applies to:** `modelDocument` in `internal/store`, and a store test that a model without a window encodes no
  such key.

### Codex gets the window in the entry that describes the model

- **Rests on:** Codex takes a model's window from the catalog entry (`context_window`), lowers the configuration
  key to the entry's `max_context_window`, counts 95% of the window as usable and compacts at 90% of it
  (`codex-rs/protocol/src/openai_models.rs`, `models-manager/src/model_info.rs`). Every bundled GPT-6.x entry has a
  ceiling of 872,000. Codex 0.171.0 loaded entries with raised window fields unchanged in an offline run. OpenAI
  documents a window of 1,050,000 and a largest input of 922,000 for those families.
- **Considered:** the key `model_context_window` in `config.toml`, which stops at 872,000; a window of 1,052,632
  for a usable 1,000,000, which starts compaction above the documented largest input.
- **Chosen because:** AIGW already writes the table and the selected model's alias. Setting the two window fields of
  that one entry to the declared window gives Codex 950,000 usable tokens for a window of 1,000,000 and compaction
  at 900,000, below the documented largest input. Where the model's ID is itself an entry of the table, that entry
  carries the window in AIGW's copy, which Codex reads only while AIGW's provider is selected.
- **Applies to:** `modelcatalog.Document.Project` and the catalog plan in `internal/adapter/codex`, and the tracked
  verifier `tools/codex/catalogcheck`, which until now proved that an alias equals its family's entry in every field
  but its ID and display name. Codex 0.171.0 rendered a raised window on one of its own entries exactly as written
  on 2026-11-22, with every other field and entry unchanged.

### A Codex model without an entry keeps Codex's fallback, and AIGW says so

- **Rests on:** for a slug that no entry matches, Codex builds its own descriptor with a window and a ceiling of
  272,000 (`model_info_from_slug`), so the configuration key cannot raise it. A catalog entry must carry
  instructions, and the fallback's are the client's built-in base prompt, which no command of the client prints. In
  an empty home, `codex debug models -c 'model="<unknown>"'` printed the bundled table and no entry for the model.
- **Considered:** an entry copied from another family, which guesses tool and instruction metadata; the client's
  base prompt vendored into AIGW, which drifts with every Codex release.
- **Chosen because:** AIGW's table holds only what Codex's own table says about a family. The cap is the client's,
  so AIGW reports it where the operator checks the client and writes nothing that pretends otherwise.
- **Applies to:** the catalog plan, and a check of the Codex adapter's status.

### Hermes gets each model's window under its own ID

- **Rests on:** Hermes resolves `providers.<id>.models.<model>.context_length` before its own tables and does not
  lower it; `models` may be a mapping from ID or a list. A window on the provider entry reaches the other entry at
  the same address, and the one in the `model` block is dropped when the model changes. AIGW's record of Hermes's
  file hashes the decoded values of its part, not their layout.
- **Considered:** the entry-level key; the `model` block's key; a list of mappings with `id`.
- **Chosen because:** only the per-model key holds one model's number through a switch of model and stays inside
  its entry. The mapping keeps each ID as a plain key, so a reader that walks the node's children still meets every
  ID. One function reads the models that an entry lists, for both shapes, and every caller uses it.
- **Applies to:** `project`, `Provider` and the readers of an entry's models in `internal/adapter/hermes`.

### The shipped team file lowers a window only on a vendor's number

- **Rests on:** each vendor's own page, read on 2026-11-22: eight families document a window below 1,000,000
  (128,000 to 512,000), and one retired family has no vendor page left.
- **Considered:** the lower of window and largest input for families whose largest input is just under 1,000,000;
  the clients' own tables.
- **Chosen because:** the owner's rule is 1,000,000 unless a real limit is lower. A vendor's documented window is
  such a limit; a largest input within a few percent of the window is not, because both clients compact well before
  it.
- **Applies to:** `manifests/team.toml` and its test. The file is the byte-identical native export and holds no
  comment, so each number's source is named in the dated qualification evidence
  (`docs/research/provider-model-qualification.md`).

## Risks / Trade-offs

- A declared window above what a provider's channel accepts → the client is refused near that limit. Hermes lowers
  the window for the session when the refusal quotes a number; the operator lowers the declaration for good.
- Codex's window fields are client metadata that a Codex release may rename → the catalog projection already
  refuses a table that it cannot parse and falls back to Codex's own; a test fails when the two fields are absent
  from the bundled table.
- The owner's installed configuration declares no window, so the eight families run at 1,000,000 until the shipped
  team file is applied again.

## Migration Plan

The next synchronization writes the windows. A configuration that declares no window stays readable by the previous
release. Before landing, the published previous release runs the lifecycle journey with Hermes and Codex after a
rollback from this program: it must synchronize both without a refusal.

## Open Questions

None that this Change decides. Whether a provider's Responses channel accepts more than 828,400 input tokens is a
paid measurement outside it.
