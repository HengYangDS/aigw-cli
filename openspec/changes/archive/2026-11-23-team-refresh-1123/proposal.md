## Why

The owner asked on 2026-11-22 for the shipped team file to be updated per provider and line, naming Gemini 4 Neon and
Qwen4 as examples, and confirmed that the whole file is meant. Google released Gemini 4 Neon on 2026-11-18 as a
generally available fourth Gemini 4 line, between Hydrogen and Carbon. Providers now list models that they did not
list before, three other vendors' strongest generally available models changed, and Meta's and StepFun's entries in
the file are previews. Each candidate provider model ran the three Hermes turns on the stock Hermes that CI pins.

## What Changes

- Gemini 4 Neon enters as a Google line: AIHubMix, on Responses and Chat Completions.
- Gemini 4 Carbon enters on DMXAPI, with its SSVIP channel.
- Qwen4-Max enters on UCloud, on Responses.
- DMXAPI's SSVIP channels of GPT-6.2 Astra and Sol enter.
- Meta: Muse Spark 1.3, its strongest model that is not a preview, replaces the Muse Wonder preview on UCloud and
  AIHubMix; DMXAPI lists no Spark, so its Muse Wonder preview leaves the file.
- Ant Ling: Ling-3.2-Flash enters on DMXAPI. AIHubMix keeps Ling-3.0-Flash: it lists 3.2 only as a free variant,
  which was not run.
- StepFun: the Step 5 Preview leaves DMXAPI and AIHubMix. Step 3.7 Flash, its strongest model that is not a preview,
  did not qualify (DMXAPI kept the tool's line numbers in its third answer; AIHubMix answered with an exhausted
  quota).
- Leaving because the provider no longer lists them: UCloud's Gemini 4 Carbon and AIHubMix's Mercury 2.5.
- Kept: AIHubMix's Cohere Command A and Mistral Large 3. Their successors answered "cannot be served at the moment",
  and the specification keeps a vendor's current family until its successor qualifies.
- Not entering: UCloud's Gemini 4 Hydrogen (the provider's safety filter refused the third turn) and Suhe.AI's
  Gemini 4 Argon and Carbon ("Model not found" though listed, as on 2026-11-19).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The line rule of `shipped-team-file` names each case, so this Change declares `skip_specs: true`.

## Impact

`manifests/team.toml`, the tests that pin its models and lines, the qualification record and `CHANGELOG.md`. The
program does not change; `aigw setup --from manifests/team.toml` takes the new models and keeps explicit client
selections of the replaced ones.

- **Readings of mine, recorded for the owner:** a preview is not generally available, so it does not stay in the
  file once read as one; where the vendor ranks two models of one line equally (Ling 3.1 and 3.2), the newer
  released one is taken.
- **Not run:** the recommendations are unchanged and need no client check.
