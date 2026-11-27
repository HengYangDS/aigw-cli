## Context

The line rule of `shipped-team-file` keeps, per provider, one family for each Anthropic, OpenAI and Google product line
(its newest generally available model) and one family for each other vendor (its most capable generally available
model), among the models the provider lists and that qualify there. A vendor keeps its current family until its
successor qualifies. The evidence of this refresh is in the canonical checkout's `build/evidence/team-refresh-20261127/`
(the providers' catalogs and the three-turn runs).

## Goals / Non-Goals

**Goals:** every provider's lines and vendors as the rule gives them on 2026-11-27, each new entry with three passing
Hermes turns on the stock Hermes that CI pins.

**Non-Goals:** the recommendations (unchanged); a program change; models that a provider lists only as a free or
preview variant.

## Decisions

### Gemini 4 Neon joins the two providers that list it now

- **Rests on:** the November 23 refresh made Neon a fourth Google line. On 2026-11-27 UCloud, DMXAPI and Suhe.AI list
  it. It passed on UCloud over Responses and over Chat Completions (run of 12:19 to 12:27) and on DMXAPI over
  Responses (12:34 to 12:37), with a control of DMXAPI's GPT-6.2 Sol in the second run. Suhe.AI answered "Model not
  found" at the first turn.
- **Considered:** declare both protocols on UCloud.
- **Chosen because:** the file declares one protocol where two pass, as AIHubMix's Neon and UCloud's Qwen4-Max do, and
  Responses is the protocol that Codex uses.
- **Applies to:** UCloud and DMXAPI in `manifests/team.toml`.

### Ling-3.1-Flash replaces Ling-3.0-Flash on AIHubMix

- **Rests on:** Ant Ling's developer FAQ (last updated 2026-11-24) lists Ling-3.1-Flash, its "new long-horizon
  version", with a window of 1M tokens shared between input and output, and Ling-3.0-Flash with 256K. AIHubMix lists
  `ling-3.1-flash` since this refresh, and it passed the three turns over Chat Completions.
- **Considered:** keep Ling-3.0-Flash until AIHubMix lists Ling 3.2 other than as a free variant.
- **Chosen because:** the November 23 reading takes the newer of two models that the vendor does not rank. Ling 3.2 on
  AIHubMix is a free variant, which the Non-Goals exclude and which answered HTTP 429; Ling 3.1 is the newest model of
  Ant Ling that qualifies there, and it succeeds 3.0.
- **Applies to:** AIHubMix's Ant Ling entry, its declared window (none), and the family list, which loses
  `ling-3.0-flash` because no other provider holds it.

## Risks / Trade-offs

- **One run per cell.** UCloud's Hydrogen failed the same way twice; Suhe.AI's Gemini 4 models and AIHubMix's Command
  A Plus failed with the provider's own answer. Each may pass another day; none is retried here.
- **Cells not run.** Mistral Large 4 and Step 3.7 Flash were not run again. Their providers still list them, so the
  next refresh runs them.
- **A window that a provider serves** may be lower than the vendor's: some providers serve Ling 3.1 Flash with 262,144
  tokens. AIHubMix's served window is not established, and no session near that size was run.
- **No field changes.** The file already declares `context_window`, which needs the next release; this Change adds
  and replaces models only.
