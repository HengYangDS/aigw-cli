## Context

The team file holds, per provider, the newest qualified model of each Anthropic, OpenAI and Google product line and
one general model per other vendor (`progressive-team-onboarding`). `setup-partial-tokens` adds that every provider
serving a client's recommended family is among that client's recommendations. Suhe.AI lists every model on all three
protocols; its operators' tutorial (2026-11-03) says the prefixed IDs are reverse models, priced apart from the plain
direct ones.

## Goals / Non-Goals

**Goals:**

- A person holding only a Suhe.AI Token gets every client configured from the team file.
- Each Suhe.AI model enters on the protocol on which it passed the three Hermes turns.

**Non-Goals:**

- Choosing between Suhe.AI's direct and reverse models for a person: both stay, and the recommendation follows the
  file's existing channel pattern.
- Gemini on Suhe.AI: no Gemini model it lists answered.

## Decisions

### Each model enters on the protocol that served it

- **Rests on:** the run of 2026-11-19 on the operator's Hermes: Suhe.AI answered HTTP 400 "not supported for path:
  /v1/responses" for Qwen4-Max, DeepSeek V4.1 Flash, MiniMax M2.5, GLM 5.3 Flash and both Gemini 4 models, and all
  but the two Gemini models and GLM 5.3 Flash passed on Chat Completions; the stock Hermes rerun repeats each cell.
- **Considered:** declare every protocol the catalog lists; declare only the protocol that passed.
- **Chosen because:** a listing does not prove that a protocol serves a model; the team file admits exact provider
  and protocol pairs.
- **Applies to:** `manifests/team.toml`; the qualification record names each cell.

### Recommendations follow the file's channel pattern

- **Rests on:** Claude Code and Claude Desktop recommend DMXAPI's plain Claude Opus 5.5, not its CC channel; Codex
  recommends DMXAPI's CDX channel of GPT-6.2 Sol and Hermes its plain model. The owner chose `codex-gpt-6.2-sol` for
  his own Codex on 2026-11-19.
- **Considered:** recommend Suhe.AI's reverse models everywhere for their price; recommend its direct models
  everywhere; follow the existing pattern.
- **Chosen because:** the pattern already encodes which channel each client was qualified on, and Suhe.AI comes after
  the existing choices, so it changes no one's selection that another Token serves.
- **Applies to:** the four recommendations; real-client checks of each client on its Suhe.AI recommendation.

## Risks / Trade-offs

- Suhe.AI's reverse models depend on its operators' access to the Claude Code and Codex channels, which this Change
  cannot observe; the direct models remain beside them.
