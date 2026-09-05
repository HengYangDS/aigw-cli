# DR-0005: Admit Client Adapters Explicitly

- Status: accepted
- Date: 2026-08-07
- Last amended: 2026-09-03

## Context

Protocol compatibility does not prove safe client configuration. Each client
has its own state, authentication, projection, rollback, uninstall, and runtime
boundaries. Generic discovery or configuration-shape similarity can otherwise
cause AIGW to adopt foreign IDEs or agents accidentally.

## Decision

The current admitted client set is Codex CLI and Desktop through their shared
Codex Home, plus Claude Code through its official per-user settings and
credential-helper interfaces. Missing
clients are untouched. Desktop-only GUI state, JetBrains products, MCP, ACP,
Hermes, and every other agent remain outside the current adapter boundary.

A new client requires one explicit adapter admission with configuration,
secret, rollback, uninstall, platform, and real verification evidence. Provider
support alone never admits a client.

One ordered registry is the operational authority for the complete admitted-
client lifecycle: discovery, desired configuration, projection planning,
atomic apply and rollback, native credential binding, status inspection, live
verification, withdrawal, and uninstall. Shared commands reach client behavior
through that registry; they do not reproduce Claude- or Codex-specific state
machines. Explicit client commands may expose client-specific operations, but
still delegate their effects to the admitted adapter.

The interface is shared; configuration paths, credential mechanisms, protocol
details, and client-specific policy remain encapsulated by each adapter.

## Consequences

AIGW stays focused on proven enterprise client surfaces. Future clients enter
through one narrow contract without reusing another client's private state or
adding parallel setup, repair, readiness, verification, or uninstall logic.

## Revisit Trigger

Revisit when a new client completes the adapter admission contract or Codex and
Claude Code change their authoritative configuration surfaces.
