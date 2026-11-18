## Context

The line rule (`progressive-team-onboarding`, "The shipped team file keeps one general family per other vendor")
keeps one family per product line of Anthropic, OpenAI and Google, replaced by its newest generally available model
once that model's provider models qualify. `TestTeamManifestKeepsOneFamilyPerCuratedLine` fails while a line holds
two families.

## Goals / Non-Goals

**Goals:**

- The Astra and Sol lines hold GPT-6.2 on every provider that qualifies it.
- The recommendations that named GPT-6.1 Sol name GPT-6.2 Sol.

**Non-Goals:**

- Making Astra a default: the owner answered on 2026-11-14 that the GPT default waits for the next Astra, and that
  choice is put to the owner, not made here.
- A program change.

## Decisions

### Which models enter

- **Rests on:** OpenAI's changelog of 2026-11-17 names `gpt-6.2-astra` and `gpt-6.2-sol` for the Responses API and
  Chat Completions, with tool calling on Responses. `aigw model search` at 11:55 and 12:21 CST listed both on DMXAPI
  (with CDX channels, no SSVIP), AIHubMix and UCloud. Every cell passed an isolated `read_file` turn, a tool-free
  resumed recall and a resumed `read_file` of a second file, on the stock Hermes v0.23.1 (`3b4215785882`); a
  control of GPT-6.1 Sol on DMXAPI passed in the same run. AIHubMix's Sol 6.2 called `read_file` on its first
  recall turn and passed its retry; DMXAPI and UCloud passed at the first attempt.
- **Considered:** keep 6.1 and 6 beside 6.2 until clients adopt it; replace each line on every qualified provider.
- **Chosen because:** the line rule allows one family per line, and every provider qualified.
- **Applies to:** `manifests/team.toml` and its tests.

### The recommendations follow the line

- **Rests on:** the Codex recommendation named `dmxapi/gpt-6.1-sol-cdx` before `ucloud/gpt-6.1-sol`, and Hermes
  `dmxapi/gpt-6.1-sol` before `ucloud/gpt-6.1-sol`.
- **Considered:** keep recommending 6.1, which leaves the file; name 6.2 Astra.
- **Chosen because:** a recommendation must name a model of the file, and Sol is the line the owner chose for
  Codex; Astra as a default stays the owner's decision.
- **Applies to:** `[recommendations.codex]`, `[recommendations.hermes]`.

### The daily line and effort stay in the guide

- **Rests on:** the owner's rule of 2026-11-18: for daily work each vendor's main line, below its flagship (GPT-6.2
  Sol, Claude Opus 5.5, Gemini 4 Argon), at `xhigh`, or at Argon's top level `high`; each other vendor's strongest
  model. The team file's schema 8 holds no effort, and each client owns its own setting. The defaults differ by
  layer: Anthropic's API runs Opus 5.5 at `medium`, and Codex 0.169.1 and 0.170.0 bundle `low` as the default of
  GPT-6.2 Sol and Astra (read with `codex debug models --bundled` in a private home).
- **Considered:** an effort field in the team file that AIGW writes into each client; a rule in the rollout guide.
- **Chosen because:** the owner chose on 2026-11-18 to document it first; a field would take over three clients'
  settings, and no provider's pass-through of effort is established.
- **Applies to:** `docs/guides/team-rollout.md`.

## Risks / Trade-offs

- **An operator's selection of a replaced model** keeps working: `aigw setup --from` keeps explicit selections, and
  the model stays in the operator's own configuration.
- **Codex's bundled metadata:** the pinned Codex 0.169.0 does not carry `gpt-6.2-sol`; AIGW's catalog projection
  copies the family's entry only when the bundled table holds the family, so a selection of a 6.2 model under 0.169.0
  keeps Codex's fallback. Codex 0.170.0, taken in the same lane, carries both names.
