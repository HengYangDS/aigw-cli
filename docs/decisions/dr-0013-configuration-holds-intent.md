---
subject: aigw:decision:0013
role: decision
state: canonical
decision_id: DR-0013
decision_status: accepted
relations:
  canonical_for: Configuration Holds Intent, Providers Are Endpoints or Cloud Locations
---

# DR-0013: Configuration Holds Intent, Providers Are Endpoints or Cloud Locations

- Date: 2026-11-25
- Amends: [DR-0012](dr-0012-provider-model-client-and-upgrade-without-importer.md) (what a provider is; point 4)
- Owner: AIGW maintainers

## Context

GitLab issue 1 showed AIGW writing an observation, the path of an editor extension's versioned Codex program, into
`aigw.toml` as if the operator had chosen it, and replacing it silently when it changed. On 2026-11-25 the owner asked
for the end state rather than a repair, with new agent clients (opencode, pi) and cloud model providers (AWS Bedrock
and others) in view. Two committees, each of four independent readers and a cross-examiner, with research on six
clients and three clouds and on two comparable tools, produced the design, and the owner chose it that day.

## Decision

1. **`aigw.toml` holds the operator's intent only.** What AIGW observes (where a program is, its version and digest,
   which program produced a client's model table, what a catalog lists, what a client chose) goes to the client's
   record file beside its configuration and to command output, never to `aigw.toml`.
2. **A client's program is declared, not observed.** A client section declares its program by an absolute path or by
   an editor extensions directory, one of the two. AIGW resolves it on each operation from the filesystem and never
   rewrites the declaration. A client that declares nothing adopts the program found on the PATH or at its fixed
   application path once, visibly, and that path becomes its declaration; editor extension directories are declared,
   not searched.
3. **A provider is a billing, quota and catalog scope with one access form:** protocol endpoints, or a cloud location
   (a cloud service, its surface, and the location fields that service needs). One region's one surface is one
   provider. A model's ID is the exact string sent on the wire.
4. **Identity is not an entity.** An endpoint provider uses the Token that AIGW stores under the provider's ID. A cloud
   provider binds, on this host only, a reference to an identity that the client's cloud SDK resolves; the reference
   never enters a team file. A cloud provider without a bound reference waits and is never selected.
5. **AIGW never signs a request, issues or caches a short-lived token, or proxies traffic** (DR-0001). Its credential
   reader returns only a static secret that AIGW stores.
6. **One place decides whether a selection is valid:** the client supports the provider's access form, the model's
   wire protocol, and, for a cloud provider, a bound reference. Every command that offers, accepts or checks a
   selection asks it.
7. **Each adapter declares its client's routing keys** (the keys that decide where a request goes, how it is
   authenticated, which model and which protocol) and the keys it may write. It writes no key outside that list.
   When it switches a client between modes it removes only keys it wrote; a routing key that it did not write is
   reported as shadowing AIGW, with its fix, and left alone.
8. **Every configuration shape that AIGW can write has real-client evidence:** a tracked command loads it offline with
   a client whose version is recorded.

The fields `authentication` and `model_provider` leave the configuration: the first follows from the provider's
access form, the second is the client's own vocabulary and follows from the adapter's declaration. DR-0012 point 4's
frozen view keeps its file name: every reader from 0.7.0 on reads an absent `authentication` as the provider-Token
kind, which stays true for every endpoint provider, and a cloud provider has no endpoint, so an old reader finds no
fingerprint for it and refuses as before. The reader of the schema 9 release no longer decodes the key, so the view
only shrinks.

## Alternatives Rejected

- **Keep `executable` and narrow its meaning:** rejected because an existing file's automatically written paths would
  silently become declarations.
- **An identity entity, an access-kind table and connectors built now:** rejected until the first real cloud journey;
  they rest on one day's research, and an experiment already showed Codex accepting fewer keys than AIGW assumed.
- **A token service or a signing helper for short-lived cloud credentials:** rejected by DR-0001 and DR-0011; the
  clients' own SDKs and refresh hooks cover every client and cloud the research found except protocol gaps.
- **A generic adapter engine with path-level ownership now:** deferred until the first new client, when Codex's TOML
  and Hermes's YAML are tried on it before any adapter moves.

## Consequences

`aigw.toml` moves to schema 9 in one release. The Codex client-native path, the only cloud path today, is removed;
cloud providers return by this record's rules when the owner has a real journey. pi is the next candidate client;
opencode waits for an upstream credential command.
