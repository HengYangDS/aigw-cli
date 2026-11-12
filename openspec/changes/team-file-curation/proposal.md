## Why

On 2026-11-12 the owner set the curation of the shipped team file: "claude 系列：fable 5.2, opus 5.5, sonnet 5.5,
haiku 5.5; gpt 系列：sol 6.1, terra 6.1；其他各家：最强模型", then "gemini-4 的可以都纳入" and "claude, gpt, gemini
按照我指定的，其他各家取最强". The team file lacks Claude Fable 5.2 and Gemini 4 Carbon. The specification still curates
only Anthropic and OpenAI, and it gives Google one general family. The file also holds two Doubao families, which the
one-family rule forbids.

## What Changes

- The specification curates the families of Anthropic, OpenAI and Google by the owner's list. Every other vendor keeps
  one general family, its most capable generally available model.
- The team file gains Claude Fable 5.2 and every Gemini 4 model that its providers serve, channel variants included,
  each after qualification.
- Every other vendor's family is checked against the vendor's release notes and the providers' catalogs. A qualified
  successor replaces the family, and Doubao has one family again.
- The two plain-DMXAPI models that waited for qualification, `gpt-6.1-terra` and `claude-haiku-5-5`, enter once they
  qualify.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `progressive-team-onboarding`: the owner curates Anthropic, OpenAI and Google.

## Impact

`manifests/team.toml`, its `setup --from` test, and `docs/research/provider-model-qualification.md`. No code changes.
Each new model is qualified with a native Hermes tool turn and a resumed recall on each of its provider and protocol
pairs. Release 0.7.1.
