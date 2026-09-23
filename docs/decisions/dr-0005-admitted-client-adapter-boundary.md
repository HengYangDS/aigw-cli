# DR-0005: Admit Client Adapters Explicitly

- Status: accepted
- Date: 2026-08-07
- Last amended: 2026-09-23

## Context

Protocol compatibility does not prove safe client configuration. Each client
has its own state, authentication, projection, rollback, uninstall, and runtime
boundaries. Generic discovery or configuration-shape similarity can otherwise
cause AIGW to adopt foreign IDEs or agents accidentally.

## Decision

The current source-level client registry is Codex CLI and Desktop through their
shared Codex Home, Claude Code through its official per-user settings, Hermes
through its native provider configuration, and Claude Desktop through its
independent third-party inference library. Missing clients are untouched. This
does not grant stable release qualification by itself. Native Claude Desktop
2.7032.0 evidence qualifies Chat, Cowork, and Code after restart on macOS;
Cowork's isolated VM tool execution remains unavailable while that VM stays in
`booting`, and Windows host qualification remains open. Session state,
unrelated GUI preferences, JetBrains products, and external services retain
their owners.

A new client requires one explicit adapter admission with configuration,
secret, rollback, uninstall, platform, and real verification evidence. Provider
support alone never admits a client.

Claude Desktop's documented third-party inference configuration is distinct
from Claude Code's settings. Its Chat, Cowork, and Code capabilities must each
be qualified. The same distinction applies to ChatGPT's regular Chat and its
Codex mode. Model families are independent of client names: admission follows
the selected protocol and required behavior, not a Claude or GPT name prefix.
The [source-backed assessment](../research/provider-tooling-assessment.md#client-surfaces-and-cross-model-inference)
records the available native and maintained integration paths.

One ordered registry is the operational authority for the complete admitted-
client lifecycle: discovery, desired configuration, projection planning,
guarded commit and compensation, credential-helper projection, status inspection, live
verification, withdrawal, and uninstall. Shared commands reach client behavior
through that registry; they do not reproduce Claude- or Codex-specific state
machines. Explicit client commands may expose client-specific operations, but
still delegate their effects to the admitted adapter.

The [projection transaction](dr-0006-transactional-client-projection.md) owns
preflight, write ordering and guarded compensation; the registry does not
promise atomic visibility across separate client files.

The interface is shared; configuration paths, credential mechanisms, protocol
details, and client-specific policy remain encapsulated by each adapter.

## Consequences

AIGW stays focused on proven enterprise client surfaces. Future clients enter
through one narrow contract without reusing another client's private state or
adding parallel setup, repair, readiness, verification, or uninstall logic.

## Revisit Trigger

Revisit when a new client completes the adapter admission contract or Codex and
Claude Code change their authoritative configuration surfaces.
