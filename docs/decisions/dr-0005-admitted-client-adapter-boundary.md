# DR-0005: Admit Client Adapters Explicitly

- Status: accepted
- Date: 2026-08-07

## Context

Protocol compatibility does not prove safe client configuration. Each client
has its own state, authentication, projection, rollback, uninstall, and runtime
boundaries. Generic discovery or configuration-shape similarity can otherwise
cause AIGW to adopt foreign IDEs or agents accidentally.

## Decision

The current admitted client set is Codex CLI and Desktop through their shared
Codex Home, plus Claude Code through an AIGW-owned process launcher. Missing
clients are untouched. Desktop-only GUI state, JetBrains products, MCP, ACP,
Hermes, and every other agent remain outside the current adapter boundary.

A new client requires one explicit adapter admission with configuration,
secret, rollback, uninstall, platform, and real verification evidence. Provider
support alone never admits a client.

## Consequences

AIGW stays focused on proven enterprise client surfaces. Future clients can be
added behind a narrow adapter contract without reusing or mutating an existing
client's private state.

## Revisit Trigger

Revisit when a new client completes the adapter admission contract or Codex and
Claude Code change their authoritative configuration surfaces.
