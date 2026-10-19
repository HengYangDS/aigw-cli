# Design

## Context

Claude Desktop chooses a Code session's tool-search mode from its third-party
profile. Without `toolSearchEnabled` the session gets no tool search. With it,
a gateway provider and a bundled Claude Code that honors host tool search, the
session runs in host mode: requests add only the tool-search shape and every
other experimental beta stays suppressed. Claude Code's own
`ENABLE_TOOL_SEARCH` setting cannot lift that suppression from the user tier.

The key is profile-wide. Desktop applies it to every model in the profile's
picker, and AIGW's profile lists the selected Route followed by each other
compatible Route of the same Account. AIGW owns the profile as one encoded
document under a managed hash.

## Goals / Non-Goals

**Goals:** Desktop sessions load MCP tool schemas on demand on Routes that
qualified the host tool-search shape, and nowhere else; the evidence for that
qualification is recorded where Route evidence already lives.

**Non-Goals:** a per-model or per-session switch, which Desktop does not offer;
tool search for Claude Code terminal sessions; any change to the experimental
beta guidance; fixing a provider's prompt-cache routing.

## Decisions

### Qualification is a Route capability

A Route capability already records which behaviors an exact Account and
protocol interface qualified. `tool_search` joins that vocabulary rather than
becoming a client option, so the evidence binds to the interface that served
it and the team catalogue carries it to every operator. Only the Anthropic
interface can declare it, because the capability names that wire shape.

### One unqualified Route keeps the key absent

Because one key governs every model in the profile, AIGW writes it only when
every exposed Route declares `tool_search`. Enabling it for the selected Route
alone would send the tool-search shape to unqualified Routes chosen from the
same picker. Dropping unqualified Routes from the picker would change the
catalogue the operator selected in exchange for a context optimization.

### The profile remains one owned document

The key is encoded with the rest of the profile, so the existing managed hash,
drift refusal, rollback and withdrawal cover it without new state. A change in
any exposed Route's capabilities changes the desired profile, which makes the
Adapter re-project it.

### No admin-tier settings

Claude Code would honor `ENABLE_TOOL_SEARCH=force` from managed settings, but
an admin tier changes how Desktop sessions merge settings and seats unrelated
policy. AIGW does not own that tier.

### Cache continuity joins Claude Route qualification

The same Claude Routes serve Desktop and terminal sessions, and a Route that
returns several response signatures can lose prompt-cache continuity between
consecutive requests. Qualification therefore records the cache reads of a
multi-request journey per signature. The measurement is AIGW's; a provider-side
remedy is not.

## Risks / Trade-offs

- [Desktop may read the profile only at launch] → enable and disable already
  report that a restart may be needed before activation is claimed.
- [A provider changes its tool-search handling after qualification] → the
  record names the app, bundled Claude Code and Route; requalify after a client
  or provider change and remove the capability when the journey fails.
- [All-or-nothing enablement hides the benefit while one Route lags] → the
  operator can qualify the remaining Route or narrow the Account's catalogue.
