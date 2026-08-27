# Adapter Admission

## Boundary

AIGW distinguishes two admissions:

1. **Provider Account admission**: an upstream provider, its verified protocol
   endpoint(s), and a separate token boundary.
2. **Client Adapter admission**: a local client's safe configuration,
   authentication, verification, rollback, and uninstall behavior.

Provider support for a protocol does not admit a new client. Only proven Claude
and Codex adapters may be enabled. A model name, a shared configuration
directory, or a generic "OpenAI-compatible" claim must never bypass admission.

Provider Account and Client Adapter are independent extension axes. Ordinary
Bearer-authenticated endpoints use the Account schema; they do not require a
provider-specific package. A distinct authentication mechanism or wire
protocol requires an explicit Adapter decision instead of name-based branching.

The admitted clients live in one static registry. Status, diagnostics, profile
validation, route validation, and adapter discovery read from that registry. A
new model in an account catalog does not change it.

## Admitted clients

| Client                      | Configuration and authentication boundary                                                                   | Required account capability            |
| --------------------------- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------- |
| Claude Code                 | Official per-user settings projection; Token is read on demand through `aigw credential claude`             | Verified Anthropic-compatible endpoint |
| Codex CLI and Codex Desktop | AIGW-owned `config.toml` projection in the shared Codex Home; official `codex login --with-api-key` binding | Verified OpenAI Responses endpoint     |

Each account retains one system token. Switching profiles within an account does
not copy a token; switching accounts does not write a token into client files.

## Host-surface ownership

Client admission does not grant AIGW control over every product that can read a
Codex-shaped configuration file. Codex CLI and Codex Desktop share one default
Codex Home, and AIGW discovers that home rather than inventing a Desktop-specific
adapter. Additional Codex homes must be configured explicitly. Desktop-only GUI
settings, IDE configuration, client sessions, and application lifecycle remain
outside the Adapter boundary. Codex and every other client retain authority over
existing conversations, model choices, transcripts, JSONL, SQLite, and runtime
metadata.

## Candidate status

| Candidate             | Correct classification                    | Status                     |
| --------------------- | ----------------------------------------- | -------------------------- |
| Z.AI GLM coding plans | Provider Account                          | Evaluation only            |
| Gemini CLI            | Separate client adapter                   | Not admitted               |
| Qwen Code             | Separate client adapter                   | Not admitted               |
| OpenCode              | Separate client adapter                   | Not admitted               |
| Pi                    | Separate client adapter                   | Not admitted               |
| Hermes Agent          | Separate client adapter                   | Not admitted               |
| Qoder                 | Client Adapter; provider surface unproved | Capability evaluation only |
| Perplexity            | Research provider, not a Codex default    | Not admitted               |
| Grok                  | Independent cross-check provider          | Not admitted               |

Official protocol documentation may prove an entry point, but it is not proof of
an AIGW adapter, tool compatibility, quality, or operational readiness.

## Required admission record

Every new adapter must supply all of the following before merge:

1. Exact client version, executable, and supported platform distribution.
2. Dedicated configuration, state, and uninstall boundaries; no Claude/Codex
   directory reuse.
3. Protocol contract: authentication, model selection, streaming, tools, and
   required image or long-context behavior.
4. Secret proof: tokens come only from the selected AIGW credential backend's
   API-token slot for the Account and never appear in public configuration,
   logs, arguments, manifests, or backups.
5. Rollback proof: byte-exact owned-state restoration; user drift fails closed.
6. User-authorized minimal real verification with non-sensitive evidence.
7. A decision covering quality, stability, cost, regional reachability,
   licensing, and maintenance burden.
8. A host-surface ownership record showing that every mutated key has one
   admitted writer and that generic discovery cannot silently adopt a foreign
   IDE or CLI surface.

The implementation must expose one cohesive Adapter boundary for discovery,
planning, guarded projection, verification, rollback, and uninstall. Client
names do not belong in provider admission, route persistence, transaction, or
presentation policy. A missing client is a successful no-op, not an invitation
to create placeholder state.

Connectivity probes are part of the protocol contract. An Anthropic probe sets
`X-Api-Key` and must not set `Authorization`; an OpenAI Responses probe sets
`Authorization: Bearer` and must not set `X-Api-Key`. Regression tests assert
both the required and forbidden headers and prove that neither the credential
nor its header name appears in command output.

Until the record is complete, the candidate remains absent from enablement,
configuration manifests, routable profiles, and automatic fallback.

## Non-default rule

Research and cross-check providers do not become Claude or Codex defaults.
Discovery in an upstream catalog is transparent but never creates an admitted
profile or route.
