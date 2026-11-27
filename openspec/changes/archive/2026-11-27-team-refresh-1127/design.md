## Context

The line rule of `shipped-team-file` keeps, per provider, one family for each Anthropic, OpenAI and Google product line
(its newest generally available model, with every channel variant that qualifies) and one family for each other
vendor (its most capable generally available model), among the models the provider lists and that qualify there. A
vendor keeps its current family until its successor qualifies. The evidence of this refresh is in the canonical
checkout's `build/evidence/team-refresh-20261127/`: the providers' catalogs and four runs of the three Hermes turns
(12:19 to 12:27, 12:34 to 12:37, 13:39 to 13:52, 14:14 to 14:26), the last three with a control of DMXAPI's GPT-6.2
Sol.

## Goals / Non-Goals

**Goals:** every line and vendor that the file holds, on every provider whose catalog of 2026-11-27 lists it, as the
rule gives them that day, each new entry with three passing Hermes turns on the stock Hermes that CI pins.

**Non-Goals:** the recommendations (unchanged); a program change; adding a vendor that the file does not hold.

## Decisions

### Listings of held lines and vendors enter where they pass

- **Rests on:** after November 23, UCloud and DMXAPI listed Gemini 4 Neon, UCloud listed Doubao Seed 2.1 Pro 260915
  (the build that the file holds on DMXAPI), and DMXAPI listed `gemini-4-carbon-ant`. AIHubMix lists LongCat 2.0, and
  no LongCat cell had run there. Each passed the three turns. Google gives Neon 1,048,576 tokens and a report of
  Meituan's API gives LongCat 2.0 1M, so neither declares a window. Suhe.AI lists Neon too and answered HTTP 400
  "Model not found".
- **Considered:** declare both protocols where both passed; leave the `-ant` channel out until DMXAPI documents it.
- **Chosen because:** the file declares one protocol where two pass, as AIHubMix's Neon and UCloud's Qwen4-Max do, and
  Responses is the protocol that Codex speaks. The `-ant` entry rests on the rule's clause that a curated line takes
  every qualifying channel variant, which the channel scenario (CC, SSVIP, CDX) does not name, and on its ID alone for
  being Gemini 4 Carbon; its exact ID keeps the channel apart in every client. A vendor's model on a provider is the
  strongest one that the provider lists and that qualifies, so AIHubMix holds LongCat 2.0 while DMXAPI holds 2.5.
- **Applies to:** UCloud's and DMXAPI's Neon, UCloud's Doubao, DMXAPI's Carbon ANT channel and AIHubMix's LongCat
  2.0, with its family.

### Ling-3.1-Flash replaces Ling-3.0-Flash on AIHubMix

- **Rests on:** Ant Ling's developer FAQ lists Ling-3.1-Flash, its "new long-horizon version", with a window of 1M
  tokens shared between input and output, and Ling-3.0-Flash with 256K. AIHubMix lists `ling-3.1-flash` since this
  refresh, and it passed the three turns over Chat Completions.
- **Considered:** keep Ling-3.0-Flash until AIHubMix's Ling 3.2 qualifies.
- **Chosen because:** the November 23 reading takes the newer of two models that the vendor does not rank. Ling 3.1 is
  newer than 3.0, and the FAQ ranks neither above the other. AIHubMix lists Ling 3.2 only as `ling-3.2-flash-free`,
  which answered HTTP 429 to five of six requests; its first call was answered on its third attempt.
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
  UCloud's Gemini 4 Hydrogen and DMXAPI's ERNIE 5.1 answered HTTP 500 "not implemented".
- **The `-ant` channel's meaning is not documented.** If DMXAPI states that it is not Gemini 4 Carbon, it leaves.
- **A window that a provider serves** may be lower than the vendor's. AIHubMix's served windows for Ling 3.1 Flash and
  LongCat 2.0 and DMXAPI's for Step 3.7 Flash are not established, and no session near any of them was run. LongCat
  2.0's 1M rests on a report of Meituan's documents, not on Meituan's own page.
- **No field changes.** The file already declares `context_window`, which needs the next release; this Change adds
  and replaces models only.
