## Why

On 2026-11-13 the owner restated the team file's curation as a rule over product lines rather than a list of
families: every Claude line (Fable, Opus, Sonnet, Haiku), every GPT line (Astra, Sol, Terra, Luna) and every Gemini
line takes its newest generally available model, and each other vendor keeps its single strongest model. A newer
model of a line replaces the older one; for example, GPT-6.2 Astra would replace GPT-6 Astra.

The file of 0.7.1 follows the owner's family list of 2026-11-12. Under the line rule it lacks GPT-6 Astra, GPT-6 Luna
and Gemini 4 Hydrogen. The owner also asked to admit UCloud's Claude Fable 5.2 quickly. It stayed out of 0.7.1 only
because Claude Desktop enables host tool search for a provider when every Claude model of it declares `tool_search`,
and Fable 5.2 had no tool-search evidence then.

## What Changes

- State the line rule in the `progressive-team-onboarding` specification, with a scenario for a newer model of a
  line.
- Add GPT-6 Astra (DMXAPI with its CDX and SSVIP channels, UCloud, AIHubMix), GPT-6 Luna (DMXAPI, UCloud, AIHubMix)
  and Gemini 4 Hydrogen (DMXAPI over Responses, AIHubMix over Chat Completions), each qualified by three Hermes
  turns on the stock Hermes that CI pins.
- Add UCloud's Claude Fable 5.2 with `tool_search`, after it answered Claude Desktop's tool-search request shape as
  UCloud's Opus 5.5 does, so Desktop keeps host tool search for UCloud.
- Add a test that fails while one curated line holds two families, and pin the new models in the team-file tests.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `progressive-team-onboarding`: the shipped team file keeps one family per product line of Anthropic, OpenAI and
  Google.

## Impact

`manifests/team.toml`, its tests, the qualification record and the CHANGELOG change. The program does not change; a
person takes the new models with `aigw setup --from manifests/team.toml`, which keeps their explicit selections.
