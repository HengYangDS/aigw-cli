## Why

The owner asked on 2026-11-27 for the shipped team file to be updated again, naming UCloud's Gemini 4 Neon, and for
its other parts as well. Since the November 23 refresh, UCloud and DMXAPI list Gemini 4 Neon, and AIHubMix lists
Ling-3.1-Flash. Each candidate provider model ran the three Hermes turns on the stock Hermes that CI pins.

## What Changes

- Gemini 4 Neon enters on UCloud and DMXAPI, on Responses. UCloud's Chat Completions passed too.
- Ant Ling: Ling-3.1-Flash replaces Ling-3.0-Flash on AIHubMix, on Chat Completions. Ant Ling documents a window of
  1M tokens for it, so it declares none; Ling-3.0-Flash declared 256,000. AIHubMix lists Ling 3.2 only as a free
  variant, which answered HTTP 429 to every request.
- Kept: AIHubMix's Cohere Command A. Command A Plus answered "cannot be served at the moment" again, and the
  specification keeps a vendor's current family until its successor qualifies.
- Not entering: UCloud's Gemini 4 Hydrogen (the provider's safety filter refused the third turn, as on November 23)
  and Suhe.AI's Gemini 4 Argon, Carbon, Hydrogen and Neon ("Model not found" though listed).
- Not run in this refresh: AIHubMix's Mistral Large 4 and Step 3.7 Flash on DMXAPI and AIHubMix, which failed on
  November 23. Mistral Large 3 stays, and StepFun stays out of the file.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The line rule of `shipped-team-file` names each case, so this Change declares `skip_specs: true`.

## Impact

`manifests/team.toml`, the tests that pin its models and lines, the qualification record and `CHANGELOG.md`. The
program does not change; `aigw setup --from manifests/team.toml` takes the new models and keeps explicit client
selections of the replaced one.

- **Not run:** the recommendations are unchanged and need no client check.
