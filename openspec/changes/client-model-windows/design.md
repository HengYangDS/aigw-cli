## Context

A provider may serve an Anthropic model under its own ID: Suhe.AI serves Claude Code's channel of Claude Opus 5.5 as
`claude-code-opus-5-5`, DMXAPI as `claude-opus-5-5-cc`. AIGW already records which Anthropic model such an ID is: its
family (`claude-opus-5-5`), from which `variantPart` derives the channel's label. Claude Code and Claude Desktop do
not read AIGW's families; each has its own way to be told.

Measured on 2026-11-20 with the real clients in an isolated home against a local endpoint that records the model of
each request (no Token, no network; `build/runtime/work/cc-model-overrides/` in the main checkout):

- Claude Code 2.1.329 with `modelOverrides` mapping the four Claude IDs to Suhe.AI's: the default model and
  `--model` with `claude-opus-5-5`, `opus`, `fable`, `sonnet` and `haiku` each sent the mapped `claude-code-*` ID,
  and `/context` gave Fable 5.2 a 1M window. Without the map, `--model opus` sent `claude-opus-5-5`.
- The same client with `model` set to `claude-code-fable-5-2`: without the map it printed that it does not
  recognise the model; with the map it did not.
- Claude Desktop 6.28899.2 still sets `CLAUDE_CODE_MAX_CONTEXT_TOKENS=200000` for every third-party session (read in
  its `app.asar`), and passes each model entry's `behavesAs` to its session as a `modelPicker` row. Its Claude Code
  2.1.328, run with that variable and `model: claude-code-fable-5-2`: 200k window without a row, 1m with the row
  `{model: claude-code-fable-5-2, behavesAs: claude-fable-5-2}`.

## Goals / Non-Goals

**Goals:**

- Every picker row and alias of Claude Code reaches a model that the selected provider serves, at that model's
  window.
- Claude Desktop's sessions run a provider's ID at the window of the Anthropic model behind it.

**Non-Goals:**

- Codex and Hermes windows; a paid near-1M measurement settles what each provider accepts first.
- Mapping a model that is not a variant of an Anthropic model ID; such a model keeps the client's handling.
- Claude Code's picker rows for models of other providers.

## Decisions

### The Anthropic model behind an ID is the model's family when that family is an Anthropic model ID

- **Rests on:** the families of every Claude model in `manifests/team.toml` and the operator's configuration are the
  Anthropic model IDs (`claude-fable-5-2`, `claude-opus-5-5`, `claude-sonnet-5-5`, `claude-haiku-5-5`), and each
  channel's ID begins or ends with its family (`variantPart`).
- **Considered:** derive the Anthropic ID from the model ID by stripping known prefixes and suffixes; keep a table of
  Anthropic model IDs in AIGW; use the family when it begins with `claude-`.
- **Chosen because:** the family already says which model an ID is, and a table would repeat it. A family that does
  not begin with `claude-` is not an Anthropic model ID, and Claude Code ignores an unknown key.
- **Applies to:** one domain function that returns, for a model, the Anthropic model ID that it serves, or nothing.

### One owner lists a provider's models for a client

- **Rests on:** Desktop's profile already listed "the selected model, then the first model of each other family";
  the map needed the same list, and the first review found the two written twice and one differing from the
  specification.
- **Considered:** keep the list in the Desktop adapter and repeat it for the map; one function in the domain.
- **Chosen because:** the map must name exactly what a picker row can select, so both read one list.
- **Applies to:** `Config.FamilyModels`, read by `Runtime.AnthropicModels` and by Desktop's profile.

### Claude Code: one `modelOverrides` map for the selected provider

- **Rests on:** the measurement above, and Claude Code's documentation: "Keys must be Anthropic model IDs ... Unknown
  keys are ignored", and overrides also apply to `--model`, `ANTHROPIC_MODEL` and the alias variables.
- **Considered:** `ANTHROPIC_DEFAULT_*_MODEL` per family (one ID per alias, no window for a pinned ID that Claude Code
  does not match); `modelPicker` rows with `behavesAs` (adds rows instead of correcting the built-in ones); the map.
- **Chosen because:** the map corrects the built-in rows and every alias in one owned value, and gives each mapped ID
  its model's window. The selected model's own ID wins its family, so the selection and its row agree; other
  families take their first variant in configuration order, as Desktop's profile does.
- **Applies to:** `internal/adapter/claude`: `modelOverrides` joins `model`, `apiKeyHelper` and the four `env` keys as
  an owned value, recorded and restored the same way; a map that would be empty is not written.

### Claude Desktop: `behavesAs` on each listed model

- **Rests on:** the measurement of Desktop's Claude Code 2.1.328 above; Desktop's schema calls the field "Claude
  model behind this ID".
- **Considered:** clear Desktop's 200K variable (not AIGW's: Desktop sets it per session); list only Anthropic IDs
  (Suhe.AI does not serve `claude-fable-5-2`).
- **Chosen because:** the field is Desktop's documented way to name the model, and its session then recognises the
  ID.
- **Applies to:** `internal/adapter/claudedesktop`: each model entry carries `behavesAs` when its family is an
  Anthropic model ID other than its own ID.

## Risks / Trade-offs

- A provider whose channel is not the Anthropic model it names would get that model's window; AIGW already treats a
  family as that claim.
- A client release could drop either field; the real-client check of each release covers both.
- Claude Desktop passes its sessions an exact list of available models as a managed setting, and with it a
  `modelOverrides` map in the user's settings is not applied: its Claude Code 2.1.328 sent `claude-opus-5-5`
  unmapped with that setting and `claude-code-opus-5-5` without it (measured 2026-11-21). So Claude Code's map
  reaches the terminal client only; a Desktop session uses the IDs of Desktop's own profile, which `behavesAs`
  covers. A Desktop session whose profile names another provider than Claude Code's selection is not rewritten.
- A person who wrote a `modelOverrides` into the settings while 0.8.3 owned them gets "changed outside AIGW" from
  the first projection of this release, since the earlier record did not cover the key; removing the key by hand
  resolves it, and `aigw sync --dry-run --json` shows it before anything is written.
- AIGW's record of Claude Code's settings gains the field `model_overrides`, and 0.8.3 refuses a record with a field
  that it does not know. A person who rolls the program back to 0.8.3 after this release first runs the command
  `aigw client disable claude` with the newer program. The owner's standing word allows breaking changes without
  compatibility code.
