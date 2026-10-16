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

## Verification

On October 16, 2026, Google's
[Gemini API changelog](https://ai.google.dev/gemini-api/docs/changelog) listed
the October 13 release of stable `gemini-4-argon`, and its
[model page](https://ai.google.dev/gemini-api/docs/models/gemini-4-argon),
updated October 14, called it the flagship reasoning model without an access
restriction. Installed AIGW 0.4.2 `aigw catalog --json` listed the exact ID on
all eight Account and protocol surfaces (DMXAPI 509, UCloud 273 and AIHubMix 698
IDs) at 19:00 +08:00; its first read at 18:58 failed on AIHubMix's three
surfaces with `request_failed`.

Hermes v0.22.2 (local `2cde9522`, with a modified tree) ran each cell from a
disposable home that borrowed the installed dependencies and used only the
Account's existing credential command. Turn 1 read an unpredictable private
file through `read_file`; turn 2 resumed the session and repeated the content
without a tool call:

| Account / protocol               | File turn | Recall turn |
| -------------------------------- | --------- | ----------- |
| AIHubMix / OpenAI Responses      | 37.9 s    | 15.2 s      |
| DMXAPI / OpenAI Responses        | 60.5 s    | 11.8 s      |
| UCloud / OpenAI Chat Completions | 14.3 s    | 16.8 s      |

A Gemini 3.8 Flash control on AIHubMix passed the same journey in 14.3 and
17.6 seconds. Every run printed Hermes' `install out of sync` warning, and the
logs held only MCP discovery retries for the empty server list. Receipts are
under `build/verification/17d9bdf66b48792221205aedb2a0a42ec801b0ac/` in the
main checkout, in `catalog-20261016/` and `argon-hermes-20261016/`.

On `17d9bdf6`, the changed `TestTeamManifestUsesRequestedLogicalModels` and
`TestTeamManifestFrontierRefreshUsesQualifiedAccountProtocols` failed as
expected: the manifest still held `gemini-3.8-flash` and its three Routes and
lacked the Gemini 4 Argon Routes. With the manifest change, the
`internal/configuration` package passes.
