## 1. Product Model and Request Paths

The design starts from the three actual product flows rather than abstract host
or agent taxonomies.

```text
AIGW control plane
  Account + Token + protocol endpoints
      + Profile(client, model)
      + Route(client -> profile)
      + native-client adapter settings

Native Codex request path (product composition)
  Codex CLI/Desktop
      -> OpenAI Responses endpoint selected by AIGW
      -> optional Codex Responses Proxy
      -> third-party OpenAI-compatible provider

Current governed deployment and acceptance path
  Codex CLI/Desktop
      -> AIGW-projected Responses endpoint
      -> Codex Responses Proxy
      -> UCloud | DMXAPI | AIHubMix

Native Claude Code request path
  AIGW-owned claude launcher
      -> process-scoped Anthropic endpoint, model, and token
      -> third-party Anthropic-compatible provider
```

The nouns have precise meanings:

| Noun | Meaning | Owner |
| --- | --- | --- |
| Provider Account | One logical credential boundary plus one or both supported protocol endpoints | AIGW configuration |
| Profile | A client-scoped model choice on one Account | AIGW configuration |
| Route | The selected Profile for `codex` or `claude` | AIGW configuration |
| Codex adapter | Projects the selected Responses endpoint/model into the shared native Codex Home and binds native credentials | AIGW |
| Claude Code adapter | Plans the native Claude process and injects the selected Anthropic endpoint/model/token only into that process | AIGW |
| Codex Responses Proxy | Optional Responses data plane for protocol normalization, replay, empty-response recovery, provider portability, and 429 backpressure | Proxy product |
| Native client runtime | Conversation/session/tool behavior after launch | Codex or Claude Code |

Codex and Claude Code are not providers and are not interchangeable adapters.
An Account may expose both protocols, allowing separate Codex and Claude Profiles
to share one credential boundary. A Profile is always scoped to exactly one
client. A Route selects a Profile; it does not select a Proxy process or mutate
the other client.

The Proxy is optional to the reusable AIGW product contract and Codex-facing
because it implements the OpenAI Responses wire contract. It is mandatory in
this closeout's deployed Codex path and runtime acceptance for UCloud, DMXAPI,
and AIHubMix. Claude Code uses the independently selected Anthropic-compatible
endpoint; it does not traverse the Responses Proxy. AIGW treats every endpoint
as operator input and never manages the selected endpoint's service lifecycle.

AIGW, Proxy, Codex, and Claude Code are the complete product graph for this
change. Unrelated applications and integrations are absent from the design.

## 2. Declarative Provider Kernel

An Account owns a label, supported protocol endpoints, one logical Token
boundary, Profile data, and explicit endpoint capabilities. An ordinary provider
is admitted through token-free manifest data and must not add a provider-specific
command, client projection branch, installer case, service manager, or core
dependency.

Provider-native account or balance diagnostics are optional leaf capabilities
behind one provider-neutral contract. Ordinary setup, selection, projection,
endpoint checking, and native-client verification work with zero diagnostics.
A diagnostic may have separate credentials and transport, but cannot become a
routing or configuration dependency.

Provider identity is never a hidden client-behavior switch. AIGW removes the
legacy Account storage flag and Azure-name workaround completely. Responses
storage, replay, and normalization belong to the selected endpoint or Proxy.

## 3. Native Client Adapters

Codex and Claude Code are the only admitted clients in this release. Each adapter
owns:

- presence and supported-version discovery;
- supported configuration or process planning;
- credential binding and least-scope injection;
- guarded AIGW-owned mutation and byte-exact rollback;
- bounded real verification when explicitly invoked;
- status/diagnostic contributions;
- uninstall of only its AIGW-owned state.

Common workflows iterate admitted adapters rather than spread client-name
switches through commands. The contract deliberately permits future native
clients such as Hermes and other agents that support third-party LLM APIs, but
none is implemented or accepted in this change. A future admission adds only
that client's adapter, one declaration, and its fixtures; it does not alter
provider policy, Proxy behavior, command roots, or another adapter.

Codex CLI and Desktop share the native Codex Home. AIGW owns only its marked
provider/model selection, sidecar, and native credential binding. It never edits
conversation JSONL, SQLite, history, Responses item records, model metadata, or
Desktop-only GUI state.

The Claude launcher is an AIGW-owned process boundary that injects the selected
Account Token only into the launched Claude Code process. Missing clients are
reported as explicit no-op skips and do not cause foreign directory creation,
installation, application launch, or partial mutation.

## 4. UX and Physical Architecture

The daily user journey is:

```text
install aigw -> aigw setup -> aigw use -> aigw check
```

Advanced Account, Profile, Route, Adapter, configuration, and update management
stays under deliberate namespaces. Failures are quiet, actionable, and free of
tracebacks, warnings, or usage banners unless requested.

The CLI root owns command composition, top-level argument binding, presentation,
and dependency assembly only. Behavior follows cohesive owners for
configuration, credentials, providers, Codex, Claude, projection/transaction,
presentation, process, and update. Generic buckets, forwarding facades, alias
packages, re-exports, one-caller abstractions, duplicated policy, and retired
compatibility owners are deleted unless an independent invariant proves their
reason to exist.

Architecture gates enforce dependency direction, package contracts, command
ownership, hard-coding, flatness, ELOC, complexity, duplication, and zero
forwarding facades. `scc` is trend evidence and ponytail review is the deletion
challenge, not another source of truth.

## 5. Repository-Owned Development Supply Chain

`go.mod` owns the exact current stable Go toolchain and dependency graph. CI,
local verification, packaging, and docs derive those versions rather than
repeat them. Repository-owned commands compose format, vet, static analysis,
architecture, portability, documentation, race tests, and package plus aggregate
coverage strictly above 95%.

Shell or Python remains only when its platform, packaging, or Forge boundary is
intrinsic. It cannot duplicate product policy or accidentally borrow another
repository or ambient Python environment. macOS, Linux, and Windows native
source/package lifecycle results remain distinct from cross-compilation.

## 6. Publication and Portability

Product source, defaults, examples, fixtures, docs, and package metadata contain
no real contributor identity, home directory, checkout path, private Forge
coordinate, credential, key, fingerprint, signer, or trust anchor. Protected
release context supplies Forge coordinates, publication actors, signers, trust,
credentials, and update-source tuples.

GitLab and GitHub keep independent verified histories. GitHub materializes
protected signer content into a permission-restricted runner-temporary file;
GitLab retains its protected file-variable contract. Repository-relative policy
paths use one host-independent lexical grammar. Required identity-neutral release
assets are deterministic and byte-equal across both Forge planes.

## 7. Acceptance and Closeout

Completion requires separate current evidence for:

- focused RED/GREEN proof and one frozen exact-HEAD local full proof;
- package and aggregate coverage each strictly above 95%;
- synthetic provider and future-client extension with bounded change radius;
- zero provider identity hacks, personal/path hard-coding, foreign-product
  dependency, forwarding facade, compatibility residue, or architecture finding;
- native macOS, Linux, and Windows verification and package lifecycle;
- trusted GitLab and GitHub histories, exact-tip green CI, signed tags/releases,
  and asset parity;
- installation only from verified release assets;
- UCloud, DMXAPI, and AIHubMix routes; native Codex traffic through the
  AIGW-projected Proxy endpoint; native Claude Code behavior; and Proxy replay,
  empty-response recovery, provider-portable normalization, and 429 backpressure;
- continuous reply in the same original affected Codex conversation without
  modifying JSONL, SQLite, history, item records, or model metadata;
- clean canonical roots and zero non-canonical lanes, worktrees, work branches,
  leases, temporary checkouts, old services, or generated residue.

Foreign-application configuration, runtime, and verification do not appear in
this acceptance set. Local proof, hosted CI, publication, installation,
runtime, exact-conversation recovery, and housekeeping remain separate claims.

## 8. Migration Plan

1. Preserve the completed hosted-CI, portable-path, native Codex Home,
   absent-client, and current-stable supply-chain fixes.
2. Add failing tests for declarative provider extension, explicit Codex
   capabilities, optional diagnostics, cohesive adapters, hard-coding, and
   forbidden cross-product coupling.
3. Reimplement required semantic-housekeeping and lane deltas under the terminal
   owners; do not mechanically overlay obsolete topology.
4. Remove redundant carriers, provider-identity hacks, shims, aliases, flat
   residue, duplicated orchestration, and stale docs/comments/docstrings.
5. Run focused tests, coverage above 95%, full local gates, strict OpenSpec, and
   pristine exact-HEAD proof.
6. Land and publish independent trusted GitLab and GitHub histories with asset
   parity; install only the verified release.
7. Verify native Codex/Claude Code, three providers, Proxy behavior, and the same
   original Codex conversation.
8. Retire every non-canonical lane through exact owner-bound closeout checks.
