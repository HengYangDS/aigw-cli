## Context

The line rule of `shipped-team-file` keeps, per provider, one family for each Anthropic, OpenAI and Google product line
(its newest generally available model, with every channel variant that qualifies) and one family for each other
vendor (its most capable generally available model), among the models the provider lists and that qualify there. A
vendor keeps its current family until its successor qualifies. The evidence of this refresh is in the canonical
checkout's `build/evidence/team-refresh-20261127/`: the providers' catalogs and three runs of the three Hermes turns
(12:19 to 12:27, 12:34 to 12:37, 13:39 to 13:52), the second and third with a control of DMXAPI's GPT-6.2 Sol.

## Goals / Non-Goals

**Goals:** every line and vendor that the file holds, on every provider, as the rule gives them on 2026-11-27, each new
entry with three passing Hermes turns on the stock Hermes that CI pins.

**Non-Goals:** the recommendations (unchanged); a program change; adding a vendor that the file does not hold.

## Decisions

### New listings of held lines and vendors enter where they pass

- **Rests on:** since November 23, UCloud and DMXAPI list Gemini 4 Neon, UCloud lists Doubao Seed 2.1 Pro 260915 (the
  build that the file holds on DMXAPI), and DMXAPI lists `gemini-4-carbon-ant`. Each passed the three turns. Suhe.AI
  lists Neon too and answered HTTP 400 "Model not found".
- **Considered:** declare both protocols where both passed; leave the `-ant` channel out until DMXAPI documents it.
- **Chosen because:** the file declares one protocol where two pass, as AIHubMix's Neon and UCloud's Qwen4-Max do, and
  Responses is the protocol that Codex speaks. The rule takes every qualifying channel variant of a curated line, and
  its exact ID keeps the channel apart in every client.
- **Applies to:** UCloud's and DMXAPI's Neon, UCloud's Doubao and DMXAPI's Carbon ANT channel.

### Ling-3.1-Flash replaces Ling-3.0-Flash on AIHubMix

- **Rests on:** Ant Ling's developer FAQ lists Ling-3.1-Flash, its "new long-horizon version", with a window of 1M
  tokens shared between input and output, and Ling-3.0-Flash with 256K. AIHubMix lists `ling-3.1-flash` since this
  refresh, and it passed the three turns over Chat Completions.
- **Considered:** keep Ling-3.0-Flash until AIHubMix's Ling 3.2 qualifies.
- **Chosen because:** the November 23 reading takes the newer of two models that the vendor does not rank. Ling 3.1 is
  newer than 3.0, and the FAQ ranks neither above the other. AIHubMix lists Ling 3.2 only as `ling-3.2-flash-free`,
  which answered its first request and then HTTP 429 on all three attempts of the next.
- **Applies to:** AIHubMix's Ant Ling entry, its declared window (none), and the family list, which loses
  `ling-3.0-flash` because no other provider holds it.

### StepFun returns with Step 3.7 Flash on DMXAPI

- **Rests on:** on November 23 Step 3.7 Flash was StepFun's strongest model that is not a preview, and it failed on
  DMXAPI and AIHubMix, so StepFun left the file. On November 27 it passed on DMXAPI over Responses. AIHubMix answered
  HTTP 402 ("You exceeded your current quota"). StepFun's model page gives it "256K tokens".
- **Considered:** wait until it qualifies on AIHubMix too.
- **Chosen because:** the rule admits a vendor's model per provider. As on November 22, a rounded figure is declared at
  its decimal reading, the lower one.
- **Applies to:** DMXAPI's Step 3.7 Flash with `context_window = 256000`, and the family list.

## Risks / Trade-offs

- **One run per cell.** A cell that failed with the provider's own answer may pass another day; none is retried here.
  UCloud's Gemini 4 Hydrogen ran only over Chat Completions, on November 23 and again here.
- **The `-ant` channel's meaning is not documented.** If DMXAPI states that it is not Gemini 4 Carbon, it leaves.
- **A window that a provider serves** may be lower than the vendor's. AIHubMix's served window for Ling 3.1 Flash and
  DMXAPI's for Step 3.7 Flash are not established, and no session near either size was run.
- **No field changes.** The file already declares `context_window`, which needs the next release; this Change adds
  and replaces models only.
