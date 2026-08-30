## Context

See [proposal](proposal.md). Credential setup already knows how to derive the
validation URL and authentication headers from a client protocol. Readiness
instead constructs an OpenAI-style `/models` request with a Bearer token for
every client, creating a second and incorrect protocol authority.

## Goals / Non-Goals

**Goals:**

- Keep one owner for client-protocol authentication probes.
- Let readiness retain bounded retry and user-facing failure classification.
- Prove Claude uses its Anthropic endpoint, `X-Api-Key`, and
  `Anthropic-Version`, while Codex retains its Responses behavior.

**Non-Goals:**

- Add provider-specific behavior or probe metadata.
- Perform a quota-consuming model request in `check`.
- Change Routes, credentials, client projections, or Proxy lifecycle.

## Decisions

1. **Credential validation owns request construction.** Expose one narrow
   request constructor from the existing credential package and use it in both
   setup validation and readiness diagnostics. This replaces duplicated URL and
   header logic without adding a package or registry.
2. **Diagnostics owns observation policy only.** It retains timeout, bounded
   authentication recovery, response classification, and redaction. It no
   longer decides protocol shape.
3. **Client declarations remain the protocol SSOT.** The constructor resolves
   the protocol through the existing admitted-client registry; unknown clients
   fail before network access.

## Risks / Trade-offs

- **A provider may not expose a standard protocol health endpoint.** Such an
  Account is not healthy for that client contract; provider-native quota probes
  remain separate optional diagnostics.
- **A future client needs a different authentication probe.** Its admission must
  extend the existing client protocol declaration and credential request owner,
  not add another readiness branch.

## Migration Plan

Ship the correction in a new prerelease. Keep rc.101 installed until the new
release passes native platform acceptance and the real workstation check; then
upgrade through the existing verified updater and retain rc.101 as rollback.
