## Context

The Codex projection already emits an absolute AIGW credential command for an
explicit native provider identity. The command implementation rejects the
Codex argument before consulting the canonical client declaration, Route, or
Token store. The existing configuration resolver and Token store already own
all required semantics.

## Goals / Non-Goals

**Goals:**

- Make the credential command follow the admitted-client declaration.
- Preserve one Route resolver and one Token authority.
- Keep every failure secret-free and fail-closed.

**Non-Goals:**

- Add a public credential-management command.
- Add provider-specific authentication or Proxy behavior.
- Change client discovery, projection, or Token storage.

## Decisions

1. **Use the admitted-client registry as the command boundary.** This removes
   the Claude-only branch and prevents a parallel list of supported clients.
   A new command tree or per-client helper was rejected because it would
   duplicate Route and Token resolution.
2. **Resolve the active client Route before reading the Token.** The existing
   configuration resolver remains the sole owner of Profile and Account
   selection. Direct Account arguments were rejected because they could diverge
   from the projected client state.
3. **Retain one hidden command.** Claude and Codex projections invoke the same
   installed executable with an explicit client argument. No public UX surface,
   compatibility alias, or credential cache is added.

## Risks / Trade-offs

- **A future admitted client may not use token-print authentication.** Its
  adapter admission must either prove this command contract or introduce a
  distinct owned authentication boundary; admission alone does not imply use.
- **A disabled adapter cannot retrieve a Token.** This is intentional: client
  configuration and credential access remain one coherent enabled state.

## Migration Plan

Ship the corrected command in the next prerelease, update through the existing
transactional installer, verify the generated Codex authentication command, and
retain the prior release as the bounded rollback target.
