## Context

The line rule of `shipped-team-file` keeps, per provider, one family for each Anthropic, OpenAI and Google product line
(its newest generally available model) and one family for each other vendor (its most capable generally available
model), among the models the provider lists and that qualify there. A vendor keeps its current family until its
successor qualifies. The evidence of this refresh is in the canonical checkout's `build/evidence/team-refresh-20261123/`
(catalogs, vendor pages, the three-turn run).

## Goals / Non-Goals

**Goals:** every provider's lines and vendors as the rule gives them on 2026-11-23, each entry with three passing Hermes
turns on the stock Hermes that CI pins.

**Non-Goals:** the recommendations (unchanged); a program change; models that a provider lists only as a free or
preview variant.

## Decisions

### Gemini 4 Neon is a fourth Google line

- **Rests on:** the Gemini API release notes, 2026-11-18: "Gemini 4 Neon (`gemini-4-neon`) generally available (GA)";
  its model page places it between Hydrogen and Carbon and replaces no model. AIHubMix alone lists it; it passed on
  Responses and on Chat Completions (run of 03:20 to 03:35).
- **Considered:** treat Neon as Argon's successor; wait for the owner to name it.
- **Chosen because:** Google names no model it replaces, Carbon, Argon and Hydrogen all remain generally available,
  and the owner named Neon as an example of what the update must hold.
- **Applies to:** AIHubMix in `manifests/team.toml` and the family list.

### A preview does not stay

- **Rests on:** Meta's model page says "Muse Wonder is in preview"; StepFun labels `step-5-preview` a preview. Their
  strongest models that are not previews are Muse Spark 1.3 and Step 3.7 Flash.
- **Considered:** keep a preview as the vendor's strongest model.
- **Chosen because:** the rule asks for the most capable generally available model, and a preview is not generally
  available. Muse Spark 1.3 passed on UCloud and AIHubMix; Step 3.7 Flash failed on both providers that list it, so
  StepFun leaves the file until it qualifies.
- **Applies to:** Meta and StepFun on every provider.

### A successor that fails keeps the vendor's current family

- **Rests on:** the rule's scenario "A newer model is not generally available or not qualified": the file keeps that
  vendor's current family. AIHubMix answered "cannot be served at the moment" for Cohere's `command-a-plus-05-2026`
  and Mistral's `mistral-large-4-0`.
- **Applies to:** AIHubMix's Cohere Command A and Mistral Large 3, which stay.

## Risks / Trade-offs

- **One run per cell.** A safety refusal (UCloud Hydrogen) and an answer format slip (DMXAPI Step 3.7 Flash) may pass
  another day; they are not retried here and stay out until a later refresh runs them again.
- **A team member on 0.9.0** applies the file as before: no field changes, only models.
