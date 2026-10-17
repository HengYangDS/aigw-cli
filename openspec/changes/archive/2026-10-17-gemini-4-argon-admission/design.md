# Design

## Context

The team manifest carries reviewed Claude and GPT Models per client and one
general Model for every other vendor. Gemini's slot holds `gemini-3.8-flash`
with three Routes: AIHubMix and DMXAPI on OpenAI Responses and UCloud on OpenAI
Chat Completions. The qualification record of October 4 to 6 kept Gemini 4
Argon out because Google offered it only to trusted testers. No recommendation
references a Gemini Route.

## Goals / Non-Goals

**Goals:** the Gemini slot holds Google's most capable generally available
general model with qualified Routes, the rule of one general Model per vendor
is a stated requirement, and the dated evidence states what was and was not
qualified.

**Non-Goals:** an independent model ranking; Codex, Claude Code, Claude
Desktop, other platforms, long-context, computer-use or sustained-availability
qualification; Routes on other Account and protocol pairs; changing another
vendor's Model.

## Decisions

### Replace the Model instead of adding one

`gemini-4-argon` replaces `gemini-3.8-flash` and its Routes. Keeping both would
give Google two general Models. Import already preserves explicit Client
Bindings and local Routes, and `--retire-route` removes an obsolete local Route
after review, so no migration code is needed.

### Keep the qualified Account and protocol pairs

The new Routes use the pairs of the replaced Routes, and each passed the same
native journey. UCloud's OpenAI Responses surface stays unused: it rejected
Gemini 3.8 Flash with `convert_request_failed` on October 5 and was not
retested. DMXAPI was qualified on its declared direct endpoint
`https://www.dmxapi.cn/v1`, not on an operator-local proxy.

## Risks / Trade-offs

- [Argon answers more slowly than Flash: the file-tool turn took 14 to 61
  seconds] → recall turns took 12 to 17 seconds; no latency guarantee is made.
- [The first AIHubMix attempt stalled for 210 seconds after its session
  started] → the retry passed; the stalled receipt is retained, not hidden.
- [An existing member keeps Gemini 3.8 Flash Routes after import] → those Routes
  keep working while their provider serves them, and the member can retire
  them with `--retire-route`.
