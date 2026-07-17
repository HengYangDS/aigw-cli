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

The admitted clients live in one static registry. Status, diagnostics, profile
validation, route validation, and adapter discovery read from that registry. A
new model in an account catalog does not change it.

## Admitted clients

| Client | Configuration and authentication boundary | Required account capability |
| --- | --- | --- |
| Claude Code | AIGW-owned shim; Anthropic environment variables exist only in the launched process | Verified Anthropic-compatible endpoint |
| Ordinary standalone Codex CLI | AIGW-owned full-selection `config.toml` projection; official `login --with-api-key` binding | Verified OpenAI Responses endpoint |

Each account retains one system token. Switching profiles within an account does
not copy a token; switching accounts does not write a token into client files.

## Host-surface exclusion and Air fallback

Client admission does not grant AIGW control over every product that can read a
Codex-shaped configuration file. On macOS, PyCharm Codex is a JetBrains AI
surface and is excluded from AIGW targets. Junie CLI remains a Junie Account /
JetBrains Account surface; it is neither executed nor admitted as a Codex
adapter. ChatGPT Desktop alone owns model choices and transcripts for existing
conversations.

JetBrains Air also remains JetBrains AI by default. It is not an ordinary Codex
adapter target. AIGW's sole exception is the explicit
`aigw route fallback air` flow: it may add a separately attributed namespaced
fallback, never a top-level AIGW selection. The operator must first inspect a
secret-free dry-run and, for apply or restore, attest that Air is idle with
`--confirm-host-idle`. This does not authorize client probing or lifecycle
control.

## Candidate status

| Candidate | Correct classification | Status |
| --- | --- | --- |
| Z.AI GLM coding plans | Separate provider account; may use the admitted Claude protocol only after account verification | Provider evaluation only |
| Gemini CLI | Separate client adapter | Not admitted |
| Qwen Code | Separate client adapter | Not admitted |
| OpenCode | Separate client adapter | Not admitted |
| Perplexity | Research provider, not a Codex default | Not admitted |
| Grok | Independent cross-check provider | Not admitted |

Official protocol documentation may prove an entry point, but it is not proof of
an AIGW adapter, tool compatibility, quality, or operational readiness.

## Required admission record

Every new adapter must supply all of the following before merge:

1. Exact client version, executable, and supported platform distribution.
2. Dedicated configuration, state, and uninstall boundaries; no Claude/Codex
   directory reuse.
3. Protocol contract: authentication, model selection, streaming, tools, and
   required image or long-context behavior.
4. Secret proof: tokens come only from `AIGW_TOKEN/<account>` and never appear
   in files, logs, arguments, manifests, or backups.
5. Rollback proof: byte-exact owned-state restoration; user drift fails closed.
6. User-authorized minimal real verification with non-sensitive evidence.
7. A decision covering quality, stability, cost, regional reachability,
   licensing, and maintenance burden.
8. A host-surface ownership record showing that every mutated key has one
   admitted writer and that generic discovery cannot silently adopt a foreign
   IDE or CLI surface.

Until the record is complete, the candidate remains absent from enablement,
team manifests, routable profiles, and automatic fallback.

## Non-default rule

Research and cross-check providers do not become Claude or Codex defaults.
Discovery in an upstream catalog is transparent but never creates an admitted
profile or route.
