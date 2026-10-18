# Claude Desktop Tool Search

## Why

Claude Desktop Code sessions on an AIGW gateway profile inline every MCP tool
schema when they start; one measured session began at 574,748 tokens, 458,557
of them MCP tool schemas. Desktop can load those schemas on demand instead.
With `toolSearchEnabled: true` in a gateway profile, its host tool search adds
only the tool-search request shape (the tool-search beta, deferred tool loading
and `tool_reference` results) while every other experimental beta stays
suppressed, as UCloud's Claude Routes require. AIGW owns that profile and its
managed hash, so the key can come only from AIGW: one added by hand or by the
Desktop settings is managed drift.

## What Changes

- Add the Route capability `tool_search`. A Route declares it on its Anthropic
  interface once a native Desktop session qualified the host tool-search shape
  there. Validation rejects it on every other interface.
- Write `toolSearchEnabled: true` into the Claude Desktop profile only when
  every Route the profile exposes declares `tool_search`, and omit it
  otherwise. The managed hash and drift refusal cover it, and a capability
  change re-projects the profile.
- Document the host tool-search contract and two qualification steps for Claude
  Routes: the Desktop tool-search journey, and prompt-cache continuity across
  consecutive requests.
- Leave the shipped catalogue unchanged until a Route passes that
  qualification.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: gains the requirement "Claude Desktop tool search
  follows Route qualification".

## Impact

AIGW reuses its Route capability vocabulary and its owned Desktop profile; it
adds no client option, state file or admin-tier managed settings, and its
`CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS` guidance is unchanged. Nothing is
removed or renamed. Claude Code terminal sessions and every other client are
unaffected, Desktop sessions stay client-owned, and Desktop may need a restart
before it reads a changed profile.

Out of scope: per-model tool search, tool search for Claude Code terminal
sessions, and the provider's cache behavior, which the account holder raises
with UCloud.
