## Context

Codex exposes codex login status, scoped by CODEX_HOME, as a public read-only
process surface. AIGW already invokes codex login --with-api-key through the
same explicit target environment for authentication mutation.

## Goals / Non-Goals

**Goals:**

- State only what current evidence proves.
- Preserve one explicit authentication mutation command.
- Keep setup and sync useful when no Token or client is installed.
- Give the operator one exact next command.

**Non-Goals:**

- Reading Codex credential files or platform keychains directly.
- Auto-authenticating during setup or sync.
- Persisting a second authentication-state cache.
- Reading or modifying conversation, model, or proxy state.

## Decisions

### Inspect the native public command, not private storage

Add a bounded login-status plan beside the existing login mutation plan. Exit
success establishes native authentication for that exact target. A non-success
result is an unready state with an actionable reason; it does not justify
parsing or probing private credential storage.

### Readiness remains decomposed

Projection, Token availability, endpoint configuration, transport, and native
authentication remain separate facts. Human rendering may summarize them, but
JSON preserves the decomposition.

### No compatibility state

No authentication marker is introduced. Every status read observes the native
client, so there is no stale shadow authority to migrate or retain.

### The declared default Codex Home is creatable

The default Codex Home remains an AIGW-owned projection surface even before its
`config.toml` exists. File presence is an observed readiness fact, not an
admission rule. When synchronization creates that file, the existing projection
sidecar records the absent pre-state so a later withdrawal removes an otherwise
empty AIGW-created file while preserving any user content added after creation.
