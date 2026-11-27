---
subject: aigw:team-file
role: reference
state: canonical
relations:
  canonical_for: the team file's fields, layout and admission rules
---

# Team File

A team distributes one reviewed, token-free [team file](../../manifests/team.toml) and applies it with
`aigw setup --from`; the [team rollout guide](team-rollout.md) covers distribution and each member's setup. This
reference defines what the file holds and what may enter it.

The team file is a curated catalog, not a collection of personal notes. It has
`version = 9`, the format of `aigw.toml` without `[clients]`, and nests each
provider's models under the provider.

## Fields

- **`[[providers]]`:** `id`, `label`, an optional `balance` probe and
  `endpoints`, one URL per protocol (`anthropic`, `responses`, `chat`).
- **`[[providers.models]]`:** the model's `id` is the provider's own model ID,
  byte for byte, preserving version punctuation and any channel suffix. The
  model is named `provider/model`, so the file never invents a name. A
  channel suffix such as `-cc` makes a separate model of the same family.
- **`family`:** optional canonical identity that variants and providers share.
  The file links a variant to its family; AIGW does not derive a family from a
  prefix.
- **`protocols`:** the protocols on which AIGW may use the model, each with the
  capabilities qualified on it. A client may select only the intersection of
  its native protocols, the provider's endpoints, and this set.
- **`label`:** optional display override. Otherwise AIGW derives
  `Provider label · Family label`; a channel model keeps an explicit
  `· CHANNEL` distinction when needed.
- **`deprecated`:** `true` marks a model that keeps existing selections but is
  not recommended.
- **`context_window`:** optional number of tokens that AIGW tells Codex and
  Hermes to use as the model's context window, a whole number from 64,000 to
  100,000,000. Without it the model has 1,000,000. Declare it only where the
  model's vendor documents a lower window, or, where no vendor document
  remains, another named source gives a lower one; use that number, the same
  for every model of the family, and name the source in the
  [dated qualification evidence](../research/provider-model-qualification.md).
  A member needs the release that introduced the field, or a later one, to
  apply a file that declares it.
- **`[[families]]`:** `id` and `label`.
- **`[recommendations.<client>]`:** sole team recommendation owner, with a
  `primary` model and optional reviewed `alternatives`; recommendations do not
  belong in display text.

## Labels and layout

Use product capitalization, dotted display versions, uppercase channel names,
and spaces around `·`. Labels contain identity, not performance promises,
review status, or instructions. The model `dmxapi/claude-opus-5-5` selects the
exact provider model ID `claude-opus-5-5`; its derived display label is
`DMXAPI · Claude Opus 5.5`. Another provider may require a differently cased
model ID without changing the family.

Use the native `aigw config export` layout: version, providers with their
models, families, then recommendations. Export writes the declared
recommendations as they are; a member's own `aigw use` selections stay local
and never become recommendations. Fields follow schema order. Export omits a
model label when it equals the derived one. Tests check the display behavior
and byte-identical native export without another formatter or model-name
registry.

Array order is display order: the file lists DMXAPI, UCloud, AIHubMix, then Suhe.AI, and
within each provider the Claude series order is Fable, Opus, Sonnet, Haiku and
the GPT order is Astra, Sol, Terra, Luna. Only identities that passed
qualification appear, the newest of each line. Missing successors are not
invented. A capability,
such as `tool_search`, sits in the model's `protocols`. Changing a protocol's
endpoint with `aigw provider edit --endpoint` clears the capability, because
it was qualified on the old endpoint.
Hermes loads the declared names and model order, but its native picker still
ranks the current provider first and other providers by model count. AIGW does
not modify that client behavior.

## Checking a provider's catalog

Verify exact provider identifiers before adding models. `aigw model search`
reads every configured catalog of each selected provider, one read per
protocol, and prints one row per model with its state: `added`, `available`,
`missing` (added, but no longer listed) or `deprecated`. A query and
`--provider` narrow the list, and `aigw model search --json` keeps each
protocol's endpoint and which protocols listed each model. Neither view
promises every model a provider can serve privately.
Anthropic Messages, OpenAI Chat Completions, and OpenAI Responses observations
are not interchangeable. A Chat Completions result cannot qualify Responses;
a Responses text call cannot qualify reasoning, tools, continuation, or
compaction. Judge a dated or suffixed identifier by the vendor's release notes,
not its name: a dated refresh such as `deepseek-v4.1-flash-1017` can be the
stronger model, while a serving tier such as `-cc` or `-free` is not.

Search compares each added model's exact provider model ID with the catalogs.
`available` and `missing` describe catalog membership only: a missing model is
reported for requalification without changing configuration. A provider
without a Token, or whose catalog cannot be read, gets one line with its fix
while the other providers still print, and the command exits 1; that is an
unknown observation, not an unavailable model.

## Admission, deprecation and retirement

Adding a model to the team file requires a successful authenticated inference
call carrying the exact provider, protocol, and model ID, plus compatibility
with the selected client and its native verification where required. A
catalog listing alone never qualifies a model. Deprecation sets
`deprecated = true` in the team file, removes the model from
recommendations, and preserves existing selections. Retirement removes the
model explicitly, with `aigw model remove` locally, after no selection or
recommendation depends on it. A search performs none of those transitions.
