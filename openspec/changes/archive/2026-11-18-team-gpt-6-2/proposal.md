## Why

OpenAI released GPT-6.2 Astra and GPT-6.2 Sol on 2026-11-17 (its API changelog and ChatGPT's "What's new"); Terra
stays at 6.1 and Luna at 6. The owner's line rule of 2026-11-13 replaces a line's family when a newer generally
available model of it qualifies, and the owner asked on 2026-11-18 to update the team file for GPT-6.2. All three
providers list both models, and each provider model passed the three Hermes turns on the stock Hermes that CI pins.

## What Changes

- GPT-6.2 Astra replaces GPT-6 Astra: DMXAPI (with its CDX channel), UCloud and AIHubMix.
- GPT-6.2 Sol replaces GPT-6.1 Sol: DMXAPI (with its CDX channel), UCloud and AIHubMix.
- DMXAPI's SSVIP channels of both lines leave the file: DMXAPI lists no SSVIP model of 6.2.
- The Codex and Hermes recommendations follow the Sol line to 6.2, with the same providers and channels.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The line rule of `progressive-team-onboarding` already names this case, with GPT-6.2 Astra as its example,
so this Change declares `skip_specs: true`.

## Impact

`manifests/team.toml`, the tests that pin its GPT models and recommendations, the qualification record, the team
rollout guide, and `CHANGELOG.md`. The program does not change; `aigw setup --from manifests/team.toml` takes the
new models and keeps explicit client selections of the replaced ones.
