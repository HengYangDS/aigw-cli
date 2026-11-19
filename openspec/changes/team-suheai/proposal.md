## Why

The owner added Suhe.AI (`https://suheai.cn`) as a provider on 2026-11-18 and asked on 2026-11-19 why the team file
still lacked it. Suhe.AI serves each model under two names: the plain ID, a direct model, and a client-prefixed ID
(`claude-code-` for Claude, `codex-` for GPT), a reverse model priced apart (its operators' tutorial of 2026-11-03).
On 2026-11-19 its catalog listed 125 models on all three protocols; the per-provider line rule picks its candidates,
and each passed the three Hermes turns on the stock Hermes that CI pins.

## What Changes

- Suhe.AI joins the team file with its endpoints for Anthropic Messages, OpenAI Responses and Chat Completions.
- Claude Fable 5.2 (prefixed only), Opus 5.5, Sonnet 5.5 and Haiku 5.5, each plain and prefixed where served, on
  Anthropic Messages; GPT-6.2 Astra, GPT-6.2 Sol, GPT-6.1 Terra and GPT-6 Luna, plain and prefixed, on Responses.
- One general model per other vendor that qualified: Kimi K3.1 Pro on Responses; Qwen4-Max, DeepSeek V4.1 Flash,
  MiniMax M2.5 and GLM 5.2 on Chat Completions, since Suhe.AI refuses them on Responses. Gemini 4 Carbon and Argon
  stay out ("Model not found" though listed), and GLM 5.3 Flash (refused upstream) gives way to GLM 5.2.
- Each prefixed model carries an explicit label (`Suhe.AI · Claude Opus 5.5 · Claude Code`), since its derived label
  would equal the plain model's.
- Every client recommends a Suhe.AI model after its other choices, as `setup-partial-tokens` requires of every
  provider that serves the recommended family: Claude Code and Claude Desktop the plain Claude Opus 5.5, Codex the
  prefixed GPT-6.2 Sol, Hermes the plain GPT-6.2 Sol.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The per-provider line rule and the recommendation coverage of `progressive-team-onboarding`, as
`setup-partial-tokens` states it, already name this case, so this Change declares `skip_specs: true`.

## Impact

`manifests/team.toml`, the qualification record, the team rollout guide and `CHANGELOG.md`. The program does not
change; `aigw setup --from manifests/team.toml` adds Suhe.AI and keeps explicit client selections. A person who holds
only a Suhe.AI Token now gets every client configured from it.
