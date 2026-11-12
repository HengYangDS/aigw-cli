## Context

The shipped team file `manifests/team.toml` names the providers, models, families and recommendations that
`aigw setup --from` applies. On 2026-11-12 the owner set its curation: Claude Fable 5.2, Opus 5.5, Sonnet 5.5 and
Haiku 5.5; GPT-6.1 Sol and Terra; every Gemini 4 model; every other vendor by its most capable generally available
model.

## Goals / Non-Goals

**Goals:** the file holds exactly the owner's Anthropic, OpenAI and Google families, and one general family for each
other vendor, each with every provider model that qualifies.

**Non-Goals:** changing the recommendations, which stay as they are, or adding providers.

## Decisions

### The owner names three vendors' families

Anthropic, OpenAI and Google keep the families the owner names. Each such family brings every provider model that
qualifies, channel variants such as DMXAPI's CC, SSVIP and CDX included. Every other vendor keeps one general family,
its most capable generally available model.

### Qualification

A model enters only after a native Hermes tool turn and a resumed recall pass on each of its provider and protocol
pairs. Each run uses a disposable Hermes home with the provider's own credential command, so no Token is copied. A
model that fails stays out and is recorded.

### The most capable model

The vendor's release notes decide which model is most capable, not the model ID. A model limited to selected testers,
a preview without general availability, or a speed or price tier of the same model does not replace a general model.
Doubao keeps one family, the most capable one that a provider serves and that qualifies.

## Risks / Trade-offs

- [Qualification spends provider quota] → two short turns per model and protocol.
- [A provider withdraws a model later] → `aigw model search` reports it as missing, and the next refresh removes it.
