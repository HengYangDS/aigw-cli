# Adapter Admission

## Boundary

AIGW distinguishes two admissions:

1. **Provider Account admission**: an upstream provider, its verified protocol
   endpoint(s), and a separate token boundary.
2. **Client Adapter admission**: a local client's safe configuration,
   authentication, verification, rollback, and uninstall behavior.

Provider support for a protocol does not admit a new client. Only Adapters in
the operational registry may be enabled. That registry records implementation
and acceptance; it does not restrict which clients a requested Change must
deliver. Model names and shared directories do not establish admission.

Provider Account and Client Adapter are independent extension axes. Ordinary
Responses Bearer authentication and Anthropic API-key authentication use the
Account schema, not provider-specific packages. Prefer an admitted client's
existing credential chain and signing through explicit client-native Profile
authentication. Unsupported authentication or wire behavior needs a reviewed
extension at its own boundary, not provider-name branching or implicit client
admission.

Classify an extension through the
[architecture's extension model](../architecture/authority-and-projection-boundary.md#extension-model)
before applying the evidence requirements below. Existing compatible Accounts
and models need configuration admission, not a new client implementation.

The admitted clients live in one static registry. Status, diagnostics, Profile
compatibility, binding validation, and client discovery read from that registry. A
new model in an account catalog does not change it.

## Dependency admission

Apply the shared
[dependency and framework admission policy](change-and-release-policy.md#dependency-and-framework-admission).
Client admission does not grant an exception to portability, credential or
compensated-projection requirements.

## Admitted clients

The table describes the source-level operational registry. Stable release
support additionally requires the native evidence listed below; source
registration alone is not a release support claim.

| Client                      | Configuration and authentication boundary                                                                                                                              | Required account capability            |
| --------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------- |
| Claude Code                 | Official per-user settings projection; Token is read on demand through `aigw credential claude <projection-fingerprint>`                                               | Verified Anthropic-compatible endpoint |
| Claude Desktop              | Official per-user third-party inference library; Token is read on demand through an executable helper and argument array                                               | Verified Anthropic-compatible endpoint |
| Codex CLI and Codex Desktop | AIGW-owned `config.toml` projection in the shared Codex Home; projection-matching command helper for Account Tokens; client-native authentication remains client-owned | Verified OpenAI Responses endpoint     |
| Hermes Agent                | Native provider and model configuration with an Account-scoped credential command                                                                                      | Verified declared endpoint protocol    |

An Account Token, when required, stays in the selected backend. Client-native
authentication stays with the client. Switching Profiles does not copy Tokens
into client files.

Hermes has native tool-loop evidence. Claude Desktop's source Adapter,
transactional projection, discovery, environment-backed credential helper and
withdrawal are implemented in the active Change. Its explicit enable and
disable commands report that the application must restart before activation or
deactivation is claimed. Chat, Cowork, Code, actual restart consumption,
host-version and release qualification remain incomplete. A synthetic
extension test cannot satisfy that client journey. CodeBuddy, WorkBuddy,
OpenCode, Pi, and Qoder retain the individual dispositions in the
[client assessment](../research/provider-tooling-assessment.md#client-surfaces-and-cross-model-inference).

Claude Desktop uses its own third-party inference configuration. Its Chat,
Cowork, and Code modes require separate capability observations; projecting
Claude Code settings does not establish Desktop routing. Hermes configuration,
provider authentication, and platform support likewise require their exact
native contracts. A plugin's external-process authentication type does not by
itself prove a general credential-command interface.

## Host-surface ownership

Client admission does not grant AIGW control over every product that can read a
Codex-shaped configuration file. Codex CLI and Codex Desktop share one default
Codex Home, and AIGW discovers that home rather than inventing a Desktop-specific
adapter. Additional Codex homes must be configured explicitly. Desktop-only GUI
settings, IDE configuration, client sessions, and application lifecycle remain
outside the Adapter boundary. Codex and every other client retain authority over
existing conversations, model choices, transcripts, JSONL, SQLite, and runtime
metadata.

For any desktop app, identify the actual mode before selecting a configuration
owner. ChatGPT's Codex mode and regular Chat are different acceptance surfaces.
Use documented configuration layers, preserve administrator precedence, and
report any app restart needed for activation. Session history and current
model choices remain outside AIGW's projection transaction.

## Required admission record

Every new adapter must supply all of the following before merge:

1. Exact client version, executable, and supported platform distribution.
2. Dedicated configuration, state, and uninstall boundaries; no Claude/Codex
   directory reuse.
3. Protocol contract: authentication, model selection, streaming, tools, and
   required image or long-context behavior.
4. Secret proof: Account Tokens come only from the selected AIGW backend's
   API-token slot; client-native authentication remains client-owned. Tokens
   never appear in public configuration, logs, arguments, manifests, or backups.
5. Rollback proof: byte-exact owned-state restoration; user drift fails closed.
6. User-authorized minimal real verification with non-sensitive evidence.
7. A decision covering quality, stability, cost, regional reachability,
   licensing, and maintenance burden.
8. A host-surface ownership record showing that every mutated key has one
   admitted writer and that generic discovery cannot silently adopt a foreign
   IDE or CLI surface.
9. An explicit compatibility result for the selected client, endpoint, model,
   and host: streaming, tools, cancellation, continuation, compaction, and any
   required images or hosted services. Preserve provider model mappings in
   user-visible identity; an alias is not proof of the serving model.

The implementation must expose one cohesive Adapter boundary for discovery,
planning, guarded projection, verification, rollback, and uninstall. Client
names do not belong in provider admission, Client Binding persistence, transaction, or
presentation policy. A missing client is a successful no-op, not an invitation
to create placeholder state.

Connectivity probes are part of the protocol contract. An Anthropic probe sets
`X-Api-Key` and must not set `Authorization`; an OpenAI Responses probe sets
`Authorization: Bearer` and must not set `X-Api-Key`. Regression tests assert
both the required and forbidden headers and prove that neither the credential
nor its header name appears in command output.

Until a Client Adapter completes admission, it remains outside the operational
registry and compatible Profiles. Model and Account catalogue discovery does not
create Profiles or select Client Bindings. Comparative product observations belong to
[research](../research/provider-tooling-assessment.md), not a second admission
registry in this policy.
