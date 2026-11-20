## Why

Claude Code and Claude Desktop do not know the client-specific model IDs that some providers serve. Suhe.AI serves
Claude Fable 5.2 to Claude Code as `claude-code-fable-5-2`, a cheaper channel than its plain ID, and does not serve
the plain `claude-fable-5-2` at all. On 2026-11-20 three faults followed from that, each measured:

- Claude Code's picker row "Fable 5.2" sent `claude-fable-5-2`, and Suhe.AI answered "400 Model not found" (the
  yheng-c2 session's handoff of 2026-11-20).
- Claude Code showed the selected `claude-code-opus-5-5` as "Custom model" (the owner's screenshot, 2.1.329).
- Claude Desktop compacted sessions on `claude-code-fable-5-2` at 167k to 200k tokens: in third-party mode it sets
  `CLAUDE_CODE_MAX_CONTEXT_TOKENS=200000` for every session, and its Claude Code applies that only to a model ID that
  it does not recognise (read in Desktop 6.6395.1 and its Claude Code 2.1.326; the supply-chain session's logs).

The owner asked on 2026-11-20 that every model run at its full 1M window in every client.

## What Changes

- AIGW writes Claude Code's `modelOverrides`: for each Claude model of the selected provider that is a variant of
  an Anthropic model, the Anthropic model ID maps to the provider's ID. The selected model's own channel wins its
  family; among other variants of one family, the first in configuration order. Claude Code then sends the
  provider's ID for every picker row and alias and gives each its 1M window.
- AIGW writes `behavesAs` on each Claude Desktop model entry whose ID is a variant of an Anthropic model, naming that
  model, so Desktop's session recognises it at its full window.
- Both are owned values: AIGW records and removes them as it does `model`, and an edit of them outside AIGW is
  managed drift.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `client-adapters`: Claude Code and Claude Desktop name the Anthropic model behind a provider's model ID.

## Impact

`internal/adapter/claude`, `internal/adapter/claudedesktop` and one domain function that finds the Anthropic model
behind an ID. No configuration schema changes; the next synchronization writes the new values. Not in scope: Codex
and Hermes windows (their own Change, after the paid measurement of near-1M requests), the curation of models per
line, and auxiliary model slots.
