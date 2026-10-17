# Gemini 4 Argon Admission

## Why

The shipped team catalogue keeps one general Model for each vendor other than
Anthropic and OpenAI: the vendor's most capable generally available general
model whose Routes qualify. Google's slot holds Gemini 3.8 Flash, selected on
October 5, 2026, while Gemini 4 Argon was limited to trusted testers.

On October 13 Google released `gemini-4-argon` as a stable model, which its
Gemini API documentation calls its flagship reasoning model. On October 16 the
authenticated AIHubMix, DMXAPI and UCloud catalogues listed that exact ID.
Native Hermes then completed a file-tool loop and same-session recall through
the three Account and protocol pairs that carry Gemini 3.8 Flash today.

## What Changes

- The team manifest replaces Model `gemini-3.8-flash` with `gemini-4-argon`,
  labelled "Gemini 4 Argon", and its three Routes with
  `aihubmix-gemini-4-argon` and `dmxapi-gemini-4-argon` on OpenAI Responses and
  `ucloud-gemini-4-argon` on OpenAI Chat Completions, each with the exact wire
  ID `gemini-4-argon`.
- The rule of one general Model per vendor becomes a `product-control-plane`
  requirement: a qualified successor replaces the vendor's Model instead of
  adding a second one.
- The qualification record keeps the October 16 observations and their limits.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: gains the requirement "The shipped catalogue keeps
  one general Model per other vendor".

## Impact

Data and documentation only: `manifests/team.toml`, its test in
`internal/configuration`, the qualification record and the CHANGELOG. No client
recommends a Gemini Route, so recommendations are unchanged. Import preserves
explicit Client Bindings, and an existing member keeps local Gemini 3.8 Flash
Routes until retiring them by importing with `--retire-route <id>`.

Authority boundary: qualification used only each Account's existing credential
command through native Hermes; no Token value was read or recorded. Breaking
removals: none for existing configurations; new imports no longer receive
Gemini 3.8 Flash Routes. Reuse: the existing manifest schema and import
retirement; no new field.

Non-goals: an independent ranking of vendor models; qualifying Gemini 4 Argon
for Codex, the Claude clients, other platforms, long contexts or computer use;
changing another vendor's Model.
