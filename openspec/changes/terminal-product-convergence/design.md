## Context

See [proposal.md](proposal.md) for motivation. AIGW currently has a working
signed release and substantial contracts for Accounts, Profiles, per-client
Routes, credential backends, Codex and Claude projections, native packaging,
and dual-Forge delivery. The remaining risk is not absence of machinery but
overlapping semantics, uneven proof, repository topology that reflects past
implementation steps, and generic checks maintained beside mature tools.

The current installed product is the rollback boundary. Source migration must
not mutate user configuration, credentials, installed clients, or an external
gateway until a released candidate has passed the corresponding acceptance
journey.

## Goals / Non-Goals

**Goals:**

- Make the path from team manifest to one usable client predictable even when
  other Providers, clients, native stores, or Forges are absent.
- Give each behavior, policy, state transition, file family, and proof one
  semantic owner.
- Reduce source, configuration, and operational entities while increasing
  coverage and native evidence.
- Make a normal Provider extension data-only and a client extension one narrow
  Adapter plus a common conformance suite.
- Deliver one signed object through local, GitHub, and GitLab surfaces without
  confusing transport identity with product identity.

**Non-Goals:**

- AIGW will not carry model traffic, supervise another product, own client
  history, or reproduce ETHOS repository lifecycle.
- This Change will not adopt a second task runner, package manager, workflow
  model, credential database, or evidence store.
- Historical intermediate artifacts will not be retained merely because they
  once existed.

## Authority inventory

The inventory is rule-based rather than a second per-file ledger. Every tracked
path inherits the responsibility of its longest matching carrier in
`.config/checks/architecture/policy.toml`; Go code narrows further through the
declared package topology and import direction. An unlisted peer carrier is a
gate failure. A generated projection is never authoritative and points back to
the source named below.

| Surface                                                   | Semantic owner and source of truth                                                            | Consumers and dependency direction                                               | Change and retirement rule                                                                                 |
| --------------------------------------------------------- | --------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| Product behavior                                          | The narrow `internal` domain package; `cmd/aigw` only composes it                             | CLI journey → domain → client or Provider leaf                                   | Change for a product invariant; retire when no invariant, caller, or test consumes it                      |
| Public commands, options, and results                     | The matching `internal/cli/<journey>` constructor and result type                             | Human and JSON clients → CLI owner → domain owner                                | Change with the journey contract; retire together when the journey is removed                              |
| Configuration, manifest fields, and environment variables | `internal/configuration`, the reviewed manifest, or the domain package declaring the variable | Input → validation → typed model → transactional projection                      | Change only with the owning schema or capability; retire after supported state has no reader               |
| Client and native resources                               | The admitted client Adapter plus `internal/platform` for OS-neutral facts                     | Configuration → complete plan → atomic OS/client projection                      | Change for an admitted client contract; remove exactly owned state on disable or uninstall                 |
| Repository tools and quality policy                       | The semantic `tools/<concern>` package plus `.config/checks/<concern>`                        | Policy → focused tool → `mise` gate → CI                                         | Change for a measured repository risk; retire when a mature owner supersedes it or the risk disappears     |
| CI projections                                            | `.config/ci/pipeline.cue`                                                                     | CUE model → `.github/workflows/*` and `.gitlab-ci.yml` → Forge runners           | Change in CUE; regenerate projections; retire a job when it proves no unique fact                          |
| Release identity and artifacts                            | `VERSION`, `CHANGELOG.md`, `.config/release`, and `tools/release`                             | Version and source object → deterministic artifacts → Forge Releases → installer | Change once per release identity; retire unreferenced failed artifacts and obsolete tags                   |
| Product intent, documentation, and journeys               | Active OpenSpec requirements plus the nearest canonical documentation entry point             | Product semantics → acceptance tests and user/contributor guidance               | Reconcile with implementation in the same Change; archive intent and delete stale guidance when superseded |
| Team configuration                                        | `manifests/team.toml` without credentials                                                     | Reviewed capability → setup import → local Account, Profile, and Route state     | Change when team capability changes; retire examples or fields without an active consumer                  |
| Repository governance and local exclusions                | `.ethos`, `AGENTS.md`, Git metadata files, and `.gitignore`                                   | Repository policy → developer and agent entry points                             | Change only at the owning governance boundary; remove obsolete exceptions and host-tool residue rules      |

This resolution also covers generated host projections: Codex and Claude files
are transactional outputs of their Adapters, while build products and Forge
files are outputs of the release and CI sources above. Host caches, Tokens,
installed binaries, and Forge observations are evidence or state, never a
tracked source of truth.

### Reconciliation findings

The baseline found the following concrete disagreements. Each is assigned to
one existing closure rather than spawning another plan or compatibility path.

| Current disagreement                                                                                            | Owning closure                                |
| --------------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| `aigw check --help` still says “gateway” although the product model is endpoint-neutral                         | 11.4 CLI semantics                            |
| Direct pushes to `dev` do not currently enter either Forge verification graph                                   | 10.3 event coverage                           |
| Architecture scanning still exempts historical `records` and nested `runtime` names                             | 7.6 and 13.1 residue removal                  |
| Host-local semantic indexes other than `.serena` are not explicitly excluded                                    | 13.2 repository hygiene                       |
| Root-level CLI tests still mix several evidence scopes beside the composition root                              | 7.3 test topology                             |
| Canonical specifications still contain superseded default-route migration text until this delta is archived     | 13.6 archive and land                         |
| The work candidate and installed release both report `0.1.0-rc.110` although they are different product objects | 13.6 unique release identity before packaging |
| Evidence policy is split across a generic directory whose final audience placement is unresolved                | 11.1 and 11.7 documentation topology          |

## Decisions

### One Change, ordered semantic closures

All terminal work stays in this Change and Work Lane, but implementation moves
through ordered closures. Each closure starts with a failing observable
contract, changes one owner, runs focused proof, stages immediately, and runs a
heavy gate only after the closure is internally green. This preserves global
scope without mixing unrelated edits or repeatedly rediscovering state.

Alternative considered: many small Changes. Rejected because the user's
cross-cutting terminal invariants would fragment, duplicate migration state,
and make deletion conditional on several overlapping branches.

### Accounts, Profiles, Routes, and Adapters are the complete control model

An Account owns endpoint capability. A Profile binds an Account, one admitted
client, one model, and one authentication owner: `account-token` means AIGW owns
the Account Token, while `client-native` means the selected client and its SDK
own credentials and request signing. A Route selects one Profile for one
client. An Adapter owns only discovery and transactional projection for that
client. There is no global Profile, `use --all`, implicit cross-client fallback,
or Provider-name behavior switch.

`setup --from` imports capability and may connect a chosen subset. `use` changes
one client Route. `sync` observes only AIGW-owned Tokens plus installed clients,
then converges eligible existing Routes and projections. `status` describes
local state; `check` probes AIGW-owned Account-Token Routes but only proves local
readiness for client-native Routes; `verify` is the sole live client-owned
authentication proof. `doctor` expands the same evidence without inventing
another state model.

Alternative considered: retain a global default for convenience. Rejected
because one Profile cannot truthfully select models for clients with different
protocols and configuration surfaces.

### Credentials use one selected backend and typed purposes

Backend selection is an installation fact. Native keyrings are admitted only
after bounded non-interactive capability observation; the environment backend
is explicit and read-only; a platform-safe file fallback is selected only by
the documented automatic policy. API Tokens and Provider diagnostic credentials
use distinct typed slots in the same backend. No command searches several
stores or opens a prompt for read-only status.

Alternative considered: search native, file, and environment stores on every
read. Rejected because it creates several authorities, unpredictable prompts,
and platform-dependent results.

### External gateways remain ordinary endpoints

AIGW may project any admitted HTTP endpoint. It neither identifies nor owns the
product behind an endpoint and does not install, inspect, start, stop, update,
or uninstall it. A team manifest carries the endpoint chosen by the team;
selecting a loopback gateway is an operator-owned Account choice, not an AIGW
default.

Alternative considered: bundle or automatically configure an external gateway. Rejected
because it violates the control-plane/data-plane boundary and prevents each
product from being useful independently.

### Extensions narrow as they approach the core

Ordinary Providers are declarative. Only protocol-specific request validation
or Provider-native diagnostics may add a leaf Adapter. Cloud credentials and
request signing remain client-native whenever the admitted client already owns
that capability; AIGW does not duplicate an SDK credential chain or signer.
New clients implement a small contract for discovery, projection, credential
binding, status, rollback, verification, and uninstall, then pass the shared
conformance suite. Existing client or Provider code is not modified unless the
common contract itself changes.

Mature libraries are adopted only when measurements show net deletion or a
clear reduction in security and maintenance risk. A framework is not valuable
merely because it is popular; AIGW will not add an HTTP framework because it is
not an HTTP server.

### Semantic topology precedes physical movement

The package map is derived from product responsibilities and dependency
direction. Only then are flat suffix families, concatenated names, ambiguous
buckets, tests, tools, and documents moved. A move must reduce owners or make an
enforced boundary visible; compatibility packages and re-exports are forbidden.

### Existing ecosystem tools remain the development control plane

`mise` is the sole cross-platform entrypoint and calls Go modules, npm, and the
existing repository verification owners. Each Work Lane owns mutable `.venv`,
`.nox`, `node_modules`, build, coverage, and temporary state; lanes share only
content-addressed caches. Minimal `bootstrap`, `check`, `native`, and `release`
tasks replace command memorization without adding shell wrappers.

Pixi is not introduced because Go is the product language and the current
ecosystem locks already cover the repository. Nix and Bazel would add a second
environment or build authority without proportional value at this scale. The
decision can be revisited only if native-library solving or repository scale
changes materially.

### Quality is a positive responsibility graph

One map assigns every tracked carrier to applicable mature format, lint, type,
architecture, security, dependency, documentation, and test owners. Custom
checks remain only for AIGW-specific invariants and consume structured policy
rather than scattered literals. Quantitative limits come from protected risks
and measured distributions, not arbitrary severity. Warnings are fixed at the
owner.

### CUE owns CI semantics; Forges own syntax and capacity

One CUE graph describes facts and dependencies. GitHub Actions and GitLab CI are
generated projections. Jobs are separated by independently useful evidence:
fast quality, Go compatibility, native product journeys, release construction,
and publication. Platform proof follows real runner capability; superficial job
symmetry is not required, semantic parity is.

### Evidence is attached to claims, not accumulated as a second history

Local gates, native jobs, release assets, and installed-runtime observations
are referenced by exact commit and immutable digest. Generated reports remain
ephemeral unless a durable consumer requires them. OpenSpec records intent and
progress; Git and release objects retain history; empty evidence shells,
records directories, stale lanes, proposals, tags, runtimes, and temporary
services are deleted when they have no consumer.

### ETHOS governs repository transitions, not product behavior

AIGW uses current ETHOS status, prewrite, proof, archive, land, and retirement
commands. Missing or defective generic lifecycle behavior is reported to ETHOS
and does not become a copied AIGW state machine. Independent product work
continues when the affected transition is not on its critical path.

## Risks / Trade-offs

- **Breaking removal exposes hidden users** → search imports, configuration,
  docs, released fixtures, and current host state before deletion; provide one
  data migration only when a supported state still has a consumer.
- **Repository-wide restructuring creates noisy diffs** → move one semantic
  owner at a time, keep behavior tests green, and avoid simultaneous cosmetic
  churn.
- **Strict tools overwhelm product work** → introduce each tool with complete
  scope and zero unexplained baseline, then remove the custom checker it
  supersedes.
- **Native evidence is slow or unavailable** → keep fast portable contracts
  local, schedule independent native jobs, and report capability absence rather
  than leaving a pipeline pending or weakening the claim.
- **Credential probing triggers host UI** → separate value-free observation
  from authenticated reads and exercise the exact native API on each platform.
- **Dependency freshness breaks reproducibility** → update one locked supply
  chain closure, regenerate deterministically, and validate clean-room and
  released artifacts before replacing the installed baseline.

## Migration Plan

1. Freeze current repository, Forge, installed-program, configuration, client,
   credential-backend, and external-endpoint facts without mutating them.
2. Close the setup, credential, selection, synchronization, and readiness
   journey with failing acceptance contracts and one state vocabulary.
3. Enforce the AIGW and optional external-gateway boundary, then prove direct
   and loopback Accounts through the same public interface.
4. Converge credential backends and client Adapters, including rollback and
   uninstall of only AIGW-owned state.
5. Derive and migrate semantic packages, tests, tools, configuration, and
   documentation; delete replaced entities in the same closure.
6. Install the positive quality responsibility graph and remove redundant
   custom mechanics.
7. Add the minimal locked development tasks, advance the supply chain, and
   prove deterministic clean-room reconstruction.
8. Generate both CI projections from CUE and obtain exact-commit native evidence
   on macOS, Linux, and Windows.
9. Build a signed release candidate; verify fresh install, update from the
   retained baseline, rollback, uninstall, credential modes, and real Codex and
   Claude projection journeys.
10. Archive and land only after all tasks are proved; synchronize local and both
    Forge `main` and `dev`, publish the signed tag and identical assets, then
    remove the proposal, Work Lane, obsolete runtimes, and other owned residue.
