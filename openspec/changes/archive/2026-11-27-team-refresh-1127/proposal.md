## Why

The owner asked on 2026-11-27 for the shipped team file to be updated again, naming UCloud's Gemini 4 Neon, and for
its other parts as well. The providers' catalogs of that day were compared with those of November 23 for every line and
vendor that the file holds, on every provider. Each candidate that a cell ran went through the three Hermes turns on
the stock Hermes that CI pins.

## What Changes

- Gemini 4 Neon enters on UCloud and DMXAPI, on Responses. UCloud's Chat Completions passed too.
- DMXAPI's `gemini-4-carbon-ant` channel of Gemini 4 Carbon enters, on Responses, labelled
  `DMXAPI · Gemini 4 Carbon · ANT`. DMXAPI does not document the suffix.
- Doubao Seed 2.1 Pro 260915 enters on UCloud, which first listed that build after November 23, on Responses. Chat
  Completions passed too.
- StepFun returns: Step 3.7 Flash, its strongest model that is not a preview, enters on DMXAPI, on Responses, and
  declares StepFun's documented window of 256K as 256,000.
- Ant Ling: Ling-3.1-Flash replaces Ling-3.0-Flash on AIHubMix, on Chat Completions. Ant Ling documents a window of
  1M tokens for it, so it declares none; Ling-3.0-Flash declared 256,000.
- Meituan: LongCat 2.0, the strongest LongCat that AIHubMix lists, enters there on Chat Completions.
- Kept: AIHubMix's Cohere Command A and Mistral Large 3, whose successors failed, and GLM-5.3 on every provider that
  holds it, since Z.ai offers GLM-5.4 only in its GLM Coding Plan.
- Not entering, each from the provider's answer: UCloud's Gemini 4 Hydrogen, DMXAPI's ERNIE 5.1, Suhe.AI's four
  Gemini 4 models, AIHubMix's Step 3.7 Flash and its free Ling 3.2 Flash. Two vendors that the file does not hold,
  Agnes on DMXAPI and Thinking Machines on AIHubMix, failed their cells, so adding a vendor does not arise here.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The line rule of `shipped-team-file` covers each case, so this Change declares `skip_specs: true`. The `-ant`
entry rests on the rule's clause that a curated line takes every qualifying channel variant; the channel scenario names
only CC, SSVIP and CDX.

## Impact

`manifests/team.toml`, the tests that pin its models and families, the qualification record and `CHANGELOG.md`. The
program does not change; `aigw setup --from manifests/team.toml` takes the new models and keeps explicit client
selections of the replaced one.

- **Not run:** the recommendations are unchanged and need no client check.
