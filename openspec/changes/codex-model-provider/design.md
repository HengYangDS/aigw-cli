## Context

Codex supports a top-level `model_provider` selection, a matching
`model_providers.<id>` table, and command-backed bearer authentication. AIGW
already owns Profile resolution and an attributed Codex configuration block.
The smallest design is therefore a Profile field projected through the existing
transaction rather than a second provider catalogue or Account fallback.

## Goals / Non-Goals

**Goals:**

- Keep provider selection Profile-scoped and Codex-only.
- Preserve the current `aigw` projection byte shape by default.
- Use the installed absolute AIGW executable as Codex's credential command.
- Make explicit-provider projection, validation, transition, and removal
  idempotent and attributable.

**Non-Goals:**

- Adding Account-level native-provider mappings or compatibility aliases.
- Adding another Token backend or moving Tokens into Codex configuration.
- Installing, configuring, or supervising a Responses proxy.
- Reading or rewriting Codex conversations, history, SQLite, or model metadata.

## Decisions

### The Profile is the sole provider-selection owner

`profiles.<id>.model_provider` is optional. Absence resolves to `aigw`.
Only Codex-scoped Profiles may set it, and its identifier is restricted to the
safe Codex table-key grammar. Account-level fallback was rejected because it
would make provider choice ambiguous across Profiles sharing an Account.

### Explicit providers use native command authentication

AIGW projects the exact Account endpoint, `wire_api = "responses"`, and an
`auth` table whose absolute command is the current AIGW executable with
`["credential", "codex"]` arguments. It does not project
`requires_openai_auth`, a bearer token, or an environment-key alternative.

### Sidecar identity authorizes replacement and removal

The sidecar records a provider identity only when it differs from `aigw`.
Validation and cleanup locate the exact attributed provider table from that
identity. This prevents stale native-provider tables while preserving the
existing default sidecar shape.

### Native providers do not consume generic Codex login or AIGW model catalogue

Command authentication resolves the Token at request time. The generic `codex
login` flow and generated AIGW model catalogue therefore remain exclusive to
the default provider.

## Risks / Trade-offs

- **Unsafe table identity** -> reject before persistence or projection.
- **Credential command drift** -> validation resolves the current executable;
  synchronization rewrites the attributed block transactionally.
- **Provider transition** -> the recorded provider identity is replaced in the
  same configuration-and-sidecar transaction.

## Migration Plan

Existing Profiles omit the field and remain byte-compatible. Operators opt in
by selecting a provider on one Codex Profile. There is no compatibility reader
for Account-level provider declarations.
