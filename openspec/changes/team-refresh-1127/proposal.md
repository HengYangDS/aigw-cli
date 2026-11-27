## Why

The owner asked on 2026-11-27 for the shipped team file to be updated again, naming UCloud's Gemini 4 Neon, and for
its other parts as well. The providers' catalogs of that day were compared with those of November 23 for every line and
vendor that the file holds. Each candidate that a cell ran went through the three Hermes turns on the stock Hermes that
CI pins.

## What Changes

- Gemini 4 Neon enters on UCloud and DMXAPI, on Responses. UCloud's Chat Completions passed too.
- DMXAPI's `gemini-4-carbon-ant` channel of Gemini 4 Carbon enters, on Responses. DMXAPI documents no meaning for the
  suffix; the entry is labelled `DMXAPI · Gemini 4 Carbon · ANT`.
- Doubao Seed 2.1 Pro 260915 enters on UCloud, on Responses, which lists that build since this refresh. Chat
  Completions passed too.
- StepFun returns: Step 3.7 Flash, its strongest model that is not a preview, enters on DMXAPI, on Responses, and
  declares StepFun's documented window of 256K as 256,000.
- Ant Ling: Ling-3.1-Flash replaces Ling-3.0-Flash on AIHubMix, on Chat Completions. Ant Ling documents a window of
  1M tokens for it, so it declares none; Ling-3.0-Flash declared 256,000.
- Kept, because their successors did not qualify: AIHubMix's Cohere Command A, Mistral Large 3 and GLM-5.3. Z.ai
  offers GLM-5.4 only in its GLM Coding Plan, so it is not generally available.
- Not entering, each from the provider's answer: UCloud's Gemini 4 Hydrogen, Suhe.AI's four Gemini 4 models,
  AIHubMix's Step 3.7 Flash and its free Ling 3.2 Flash. Two vendors that the file does not hold, Agnes on DMXAPI
  and Thinking Machines on AIHubMix, failed their cells, so adding a vendor does not arise here.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The line rule of `shipped-team-file` names each case, so this Change declares `skip_specs: true`.

## Impact

`manifests/team.toml`, the tests that pin its models and families, the qualification record and `CHANGELOG.md`. The
program does not change; `aigw setup --from manifests/team.toml` takes the new models and keeps explicit client
selections of the replaced one.

- **Not run:** the recommendations are unchanged and need no client check.
