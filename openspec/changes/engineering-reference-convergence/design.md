# Design

## Context

AIGW 0.1.0 is an immutable published baseline. The repository already declares the intended product boundaries and many strong local and hosted gates, but accumulated delivery work can still leave physical topology, user journeys, tests, documentation, and quality policy harder to understand than the product requires. This Change converges those surfaces without rewriting the stable tag or importing generic lifecycle state into AIGW.

## Goals / Non-Goals

**Goals:**

- Establish one evidence-backed map from product journeys and invariants to semantic owners.
- Repair behavior before reorganizing its files, then make logical and physical ownership agree.
- Remove unconsumed entities and parallel semantics before adding tools or abstractions.
- Make setup, deferred activation, synchronization, credentials, client projection, installation, recovery, and extension natural on every supported platform.
- Make the repository independently understandable and reproducible by a new contributor.

**Non-Goals:**

- Rebuild AIGW as a traffic gateway, daemon, desktop application, or package-manager framework.
- Add a tutorial subsystem, duplicate status ledger, compatibility facade, or local ETHOS state machine.
- Rewrite 0.1.0 artifacts, tags, historical OpenSpec archives, or user-owned client configuration.
- Change Codex Responses Proxy implementation from this repository.

## Decisions

### 1. Audit by semantic closure, not by directory

Begin with the public journeys and invariants, then trace their source, tests, configuration, documentation, and evidence. Each closure ends with one owner, focused tests, full gates where needed, updated documentation, and deletion of displaced material. A file-by-file cleanup without this trace is rejected because it can polish the wrong topology.

### 2. Delete before adding

For every duplicate helper, wrapper, configuration fragment, compatibility path, or document, first identify its current consumer and protected invariant. Delete it when both are absent. Reuse an existing owner when present. Add an entity only when no current owner can express the behavior without increasing coupling; record the displaced complexity in the existing decision register.

### 3. Product journeys define dependency order

Converge the paths in this order: configuration and route authority; credentials; client projections; installation and recovery; provider/client extension; repository topology; quality graph; documentation; performance and final acceptance. This order prevents structural refactors from preserving broken behavior and avoids running expensive matrices before local semantics stabilize.

### 4. AIGW and Proxy compose only through explicit endpoints

AIGW owns Accounts, Tokens, Profiles, Routes, and client projections. It carries no Proxy lifecycle, state, or mandatory loopback default. A profile may select a direct provider endpoint or any independently managed compatible endpoint. Proxy owns protocol translation and runtime traffic when explicitly installed. Tests use an external endpoint contract rather than importing Proxy implementation.

### 5. Quality has one declarative graph

The repository keeps one machine-readable mapping from tracked carrier classes to mature formatters, linters, analyzers, tests, security checks, and generated projections. GitHub and GitLab remain deterministic projections of that graph. Custom code is retained only for AIGW-specific semantics that general tools cannot express. Threshold changes require measured distributions and named risks, not aesthetic severity.

### 6. Cross-platform claims consume real released bytes

Source tests establish contracts; native jobs establish host behavior; published-artifact jobs establish distribution behavior. macOS, Linux, and Windows each exercise build, install, update, rollback, uninstall, credential mode, and client projection using the selected immutable release bytes. Unsupported platform trust, client availability, or credential service behavior remains explicit rather than inferred.

### 7. Documentation teaches by tracing the product

The root README remains the concise product entry point. Task-oriented guides explain complete user and contributor journeys; architecture documents explain stable boundaries; decisions record chosen trade-offs; research remains evidence for future choices. Code, commands, diagrams, tables, and links are validated through the same repository quality graph. No private local file may be a shared prerequisite.

## Risks / Trade-offs

- **Large scope can create churn** → complete one semantic closure at a time and require deletion plus focused acceptance before the next structural move.
- **Stricter gates can reward fragmentation** → derive thresholds from distributions and preserve coherent domain units.
- **Native evidence can become expensive** → run focused local falsification first, freeze inputs, then reuse exact matching immutable evidence.
- **External tools can expand the maintenance surface** → admit only stable tools that replace more code and operational burden than they add.
- **Breaking cleanup can surprise existing users** → remove only unsupported or unconsumed behavior; document migration for supported public contracts.

## Accepted baseline

Subsequent comparisons use the immutable stable release rather than an RC or a
mutable checkout:

- source commit: `0e4c411410b264ab587aa90b8d237acc5e06fa79`;
- signed tag object: `ccd4853751fad10fc851207c5a348d4f8915b2c0`
  (`v0.1.0`);
- release inventory: the ten entries signed by `checksums.txt.sig`, including
  native archives for macOS, Linux, and Windows;
- peer identity: GitHub and GitLab expose the same tag object, peeled commit,
  checksum manifest, signature, and provenance bytes;
- installed product: Homebrew Cask `aigw 0.1.0` at
  `/opt/homebrew/Caskroom/aigw/0.1.0/aigw`, accepted as a notarized Developer ID
  application;
- retained user state: Keychain storage is available, Claude selects
  `ucloud-claude-fable-5-1`, Codex selects `ucloud-gpt-6-astra`, and current
  `status`, `check`, and `doctor` observations pass;
- transition evidence: GitHub workflow `35445819802` exercised
  `0.1.0-rc.118` to `0.1.0`, rollback, and forward recovery on macOS, Linux,
  and Windows; published-artifact verification passed in GitHub jobs
  `35445069751`, `35445072366`, and `35445075077`, and GitLab pipeline `7538`.

The preceding RC executable, portable installation directory, and rollback copy
are absent. Historical signed release records remain chronology, not an active
installation or compatibility path.

## Feedback acceptance map

The Change keeps one task ledger while mapping the accumulated feedback to its
owning closure:

| Feedback theme                                                                                                                             | Owning tasks |
| ------------------------------------------------------------------------------------------------------------------------------------------ | ------------ |
| Natural first setup, partial credentials, absent clients, later synchronization, and precise `use`/`check` semantics                       | 2.1–2.6      |
| Keychain, Secret Service, Credential Manager, file and environment portability without repeated prompts                                    | 3.1–3.5      |
| Optional Proxy composition, direct endpoints, and low-cost Provider or Client extension                                                    | 4.1–4.6      |
| Semantic packages, test topology, precise names and types, no suffix-based flat sprawl, hard-coding, wrappers, or parallel implementations | 5.1–5.6      |
| Comprehensive format, lint, type, test, documentation, schema, security, complexity, size, coverage, and warning policy                    | 6.1–6.7      |
| Latest stable direct supply chain, locked clean-lane bootstrap, and removal of stale installers or caches                                  | 7.1–7.5      |
| English, navigable, accurate documentation; correct research, decision, architecture, guide, governance, and operations placement          | 8.1–8.6      |
| Real macOS, Linux, and Windows product journeys; performance; dual-Forge identity and CI projection                                        | 9.1–9.7      |
| Versioning, release only for changed product bytes, branch convergence, proposal cleanup, lane retirement, and residue removal             | 10.1–10.4    |

Generic Work Lane, lease, commitment, publication, review, and retirement
mechanisms remain ETHOS responsibilities. Proxy protocol translation, service
supervision, replay correctness, and host-service cleanup remain Proxy
responsibilities. Codex Desktop rendering and conversation-model selection, and
operator-wide private configuration, remain application or workstation
responsibilities. AIGW tests only its explicit boundary with each of those
systems and does not copy their state machines into this repository.

## Initial semantic inventory

The first repository-wide inventory establishes the surfaces that later closures
must preserve or deliberately remove:

- the public command graph contains one `aigw` composition root and the setup,
  selection, readiness, credential, client, installation, recovery, catalogue,
  and provider-administration journeys listed by `aigw --help`;
- the production graph contains 43 Go packages. Its principal dependency
  boundaries run from CLI adapters into configuration, presentation, process,
  secrets, synchronization, and transaction owners; the architecture gate
  currently reports no undeclared package or import edge;
- focused behavior is co-located with its semantic owner, while cross-command,
  client, upgrade, and release journeys live in explicitly named acceptance
  packages rather than in a second implementation;
- repository configuration has distinct owners for architecture, coverage,
  dependency policy, Go analysis, size, Markdown, secret scanning, TOML,
  release construction, and the CUE CI graph. GitHub and GitLab workflow files
  are generated projections of that CI graph;
- current documentation has one index and distinct architecture, concepts,
  decisions, experience, governance, guides, operations, research, history, and
  legal domains; every current document below `docs/` has an inbound tracked
  link;
- the tracked repository contains no `evidence`, `claims`, `chronicle`,
  `.code-memory`, or `.ethos/state` directory. Serena state and installed Node
  packages are ignored Work Lane-local development state;
- after the stable delivery was accepted, both peer proposal refs were removed,
  local and peer `dev` converged on the signed object `b2fbec3a`, the superseded
  `stable-macos-release` Work Lane was retired, and this Change now owns the sole
  active Work Lane.

This is an ownership inventory, not a claim that every current package or file
is already optimal. Task 1.4 carries the consumer-level deletion audit; later
journeys may still prove that an apparent owner is redundant or misplaced.

## Accepted journey observations

Focused acceptance at the current baseline confirms the intended public
semantics before further restructuring:

- manifest setup imports capability with no Token and no installed client;
- any one available Account Token is sufficient, including an explicitly
  selected Account or the read-only environment backend;
- setup projects only the intersection of installed clients and usable Routes;
- installing a client or making its Token available later is completed by
  `aigw sync` or an explicit `aigw use <profile>`, without repeating setup;
- independent Claude and Codex selections remain independent, and `aigw check`
  accepts both without a hidden global or bulk-selection step;
- cancellation, invalid backend state, projection failure, persistence failure,
  and compensation failure preserve or restore the owned preimage and report the
  exact incomplete boundary.

The focused suites for CLI acceptance, onboarding, configuration,
synchronization, and secret backends pass at the current Work Lane base. These
observations establish the existing behavior for tasks 2.1 through 2.5. They do
not yet prove released-artifact execution on every host, which remains task 9.3.

The configuration-lifecycle audit found one public setup command, one shared
setup transaction, and no command alias or parallel persisted selection model.
The removed `recommended_default` manifest field and the preceding local
default-plus-overrides schema are rejected by strict decoding or exact version
admission; runtime code retains neither migration reader. Unsupported local and
manifest schema errors now state that AIGW does not reinterpret versions and
direct the operator to a matching release or an explicitly reviewed canonical
export instead of suggesting a generic readiness retry. Focused configuration,
presentation, onboarding, synchronization, and CLI acceptance suites pass.

Credential portability is accepted at signed commit `3863e05e`. The ordinary
GitHub review run `35455496791` exercised the complete native graph on macOS,
Linux, and Windows; Windows also completed the real Credential Manager journey.
GitLab review pipeline `7573` independently passed macOS and Linux after its
stopped OrbStack runtime was restored without changing repository code. The
focused stores verify explicit environment, file, and keyring selection,
read-only environment credentials, persisted automatic selection, precise
Secret Service unavailability, no fallback after an explicit keyring failure,
bounded worker termination, restricted identity environment, metadata-only
observation, and standard-input-only mutation. GitHub workflow `35457062367`
then consumed published `v0.1.0-rc.118` as the predecessor and passed the real
macOS Keychain create, read, rotate, update, rollback, forward-recovery,
uninstall, reinstall, and exact-delete journey. These observations establish
tasks 3.1 and 3.2; Linux proves the unavailable Secret Service path rather than
claiming a service that the runner does not provide.

Task 3.3 is accepted at signed commit `84f1edb3`. GitHub workflow
`35462782608` consumed published `v0.1.0-rc.118` and built the exact candidate
source. Its disposable macOS Keychain journey exercised setup, create, read,
rotate, staged identity migration, verified finalization, update, rollback,
forward recovery, retained Claude and Codex credential commands, uninstall,
reinstall, and exact deletion. Configuration bytes and the selected backend
remained unchanged across replacement. Current-HEAD provider-double suites
independently pass postimage-guarded replacement, backend-choice, setup, route,
account-migration, and file-recovery failure cases without accessing operator
credentials. Accepted `dev` run `35463197605` then passed the macOS, Linux,
Windows, and quality jobs at the same commit. The candidate executable differs
from the published `v0.1.0` artifact, so this is transition evidence, not a
claim that current HEAD has already been released.

Task 3.4 retains a narrower boundary than its original shorthand suggested.
The default projected credential command is owned by the active AIGW
executable. The supported `credential_command` field remains an explicit,
operator-owned extension contract: AIGW preserves it but neither installs,
discovers, owns, nor falls back to that executable. Cleanup therefore targets
only AIGW-owned duplicate identities, automatic readers, stale owned slots, and
unsupported compatibility paths.

The task 3.4 consumer and host audit finds one native implementation dependency,
`go-keyring`, behind the cross-platform bounded worker. The current macOS
Keychain exposes exactly the `aihubmix`, `dmxapi`, and `ucloud` slots under
service `AIGW_TOKEN`, matching the three configured Accounts; no test or stale
slot remains. The rejected `aigw-keychain`, `aigw_keychain.py`, and
`keychain-return` executables, source references, and launchd identities are
absent. Both installed Claude and Codex projections invoke
`/opt/homebrew/bin/aigw`, which resolves to the signed Homebrew 0.1.0 binary,
and an installed `aigw sync --dry-run --json` reports both targets already
converged. The independent `keychain-blob` executable remains outside AIGW: its
credential-governance source and tests are active consumers, and neither AIGW
configuration nor client projection selects it.

Task 3.5 uses the repository's pinned Gitleaks policy rather than a second
scanner. The current authored tree, local recovery records, GitHub runs
`35462782608` and `35463197605`, GitLab pipeline `7591`, and the recursively
opened `v0.1.0` assets from both peers contain no finding; all 12 peer assets
also have identical SHA-256 values. A reachable product-ref history scan finds
one false positive consisting of two synthetic model identifiers in an old
test. Extending the scan to all refs adds 63 false positives from ETHOS's
private attestation-set ref: 53 Git object digests and 10 structured Git object
references. The three current artifact-store findings are the public SSH
signing status. Explicit private-key and common access-token patterns are
absent. Source review confirms that projection fingerprints hash only client,
Account, and endpoint identity; artifact and ownership hashes consume program
or non-secret projection bytes, not Tokens. Current Claude sidecars record no
present original credential value, and the first-adoption path rejects
plaintext credentials or a foreign helper before writing a sidecar.

The external-gateway boundary is accepted through tasks 4.1 to 4.5. Production
source and the shipped team manifest contain no Proxy identity, fixed Proxy
port, listener, service manager, installation, or runtime lifecycle. An Account
holds either a direct HTTPS endpoint or an explicitly selected loopback endpoint;
both follow the same Profile, Route, projection, readiness, and diagnostic path.
Loopback classification reports only that the service is external, while an
absent or unavailable endpoint produces the ordinary configuration or network
failure without installation, startup, retry, or repair of another product.
Historical Proxy-shaped test ports were replaced with a neutral loopback
fixture. Strict local and manifest schema admission rejects older versions and
unknown fields rather than retaining a compatibility reader or gateway field.

A synthetic `northstar` Provider passes parse, merge, connected-Account route
selection, runtime resolution, and client-native Codex projection using only
manifest data; no provider name enters the control-plane core. The sole
provider-specific production package remains the explicitly selected DMXAPI
Account diagnostic, which is outside Route and projection semantics. A
synthetic `future` Client passes the complete registry contract for discovery,
convergence, preflight, guarded projection, change detection, inspection, live
verification, compensation, disable, and withdrawal. The same registry rejects
unadmitted or incomplete implementations, prepares all selected clients before
writing, compensates in reverse order, and preserves existing Accounts, Routes,
and built-in clients. Adapter and uninstall acceptance confirm that withdrawal
removes only owned projection state and never the foreign client executable.
The extension and admission documents identify the same contract and keep
incompatible wire behavior in an independent data plane. Focused suites for
`internal/configuration`, `internal/client`, `internal/cli/acceptance`,
`internal/cli/adapter`, `internal/cli/install`, `internal/cli/readiness`, and
`internal/codex` pass with these boundaries.

Task 4.6 compares replaceable responsibility rather than feature count. At this
closure, AIGW declares 14 direct Go modules; the selected build list contains
63 modules and the compiled command graph contains 315 packages. The installed
arm64 program is 10,770,864 bytes. The relevant product owners contain 1,448
production lines in configuration, 912 in client orchestration and adapters,
and 211 in optional Provider diagnostics. These counts bound the surface a
candidate could displace; they do not by themselves justify retention or
adoption.

The bounded upstream review on 2026-09-20 produced these decisions:

- [koanf 2.3.6](https://github.com/knadh/koanf/releases/tag/v2.3.6) declares
  three direct and one indirect core module requirement; [Viper
  1.21.0](https://github.com/spf13/viper/releases/tag/v1.21.0) declares ten
  direct and seven indirect requirements. Both own generic source loading and
  merging. Neither removes AIGW's strict schema, unknown-field rejection,
  Account/Profile/Route validation, explicit conflict admission, atomic store,
  or source-preserving client projection. Their alias, default, watch, or
  ambient-source behavior would introduce a second configuration semantic, so
  neither is adopted.
- [go-plugin 1.8.0](https://github.com/hashicorp/go-plugin/releases/tag/v1.8.0)
  declares seven direct and eight indirect requirements and adds subprocess,
  RPC, handshake, version, installation, trust, logging, and cleanup contracts.
  The static in-process registry already centralizes selection, preflight,
  compensation, inspection, verification, and withdrawal; a dynamic plugin
  layer would delete none of the client-specific ownership work. It is rejected
  unless independently shipped third-party adapters become a demonstrated
  product requirement.
- [Fx 1.24.0](https://github.com/uber-go/fx/releases/tag/v1.24.0) declares six
  direct and three indirect requirements, while [Wire
  0.7.0](https://github.com/google/wire/releases/tag/v0.7.0) is archived. AIGW's
  explicit construction has no unresolved dependency-graph or component
  lifecycle problem, so either dependency would add machinery without deleting
  a product obligation.
- [LiteLLM 1.101.0](https://github.com/BerriAI/litellm/releases/tag/v1.101.0)
  is a Python SDK and traffic gateway, not a replacement for local Account,
  credential, Route, or native-client projection ownership. It may be selected
  as an external endpoint, or evaluated later against Proxy with protocol and
  lifecycle evidence, but it must not enter AIGW as a framework dependency.

The resulting decision is to add no framework. Existing narrow dependencies
already delegate TOML parsing, native credential APIs, bounded Windows cleanup,
and CLI grammar at their exact owners. Reconsider this decision only when a
candidate proves a required behavior and deletes more implementation,
verification, platform, and operating responsibility than it introduces.

## Initial deletion inventory

The initial residue audit classifies current candidates before any removal:

| Candidate                                                             | Current classification                                               | Disposition                                                                   |
| --------------------------------------------------------------------- | -------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| RC.118 local executable, portable install root, and rollback copy     | Proved disposable after installed 0.1.0 acceptance                   | Already removed; retain no compatibility reader.                              |
| `proposal/engineering-reference-convergence` on both peers            | Review-only projection absorbed into exact `dev` object `b2fbec3a`   | Already removed by merged reviews.                                            |
| `work/20260919-stable-macos-release` and its worktree                 | Clean lane absorbed by accepted truth                                | Retired through ETHOS with exact-head receipt.                                |
| `.serena/` and `node_modules/` in the active Work Lane                | Ignored, reproducible development state                              | Keep only while the lane is active; remove with lane retirement.              |
| Git-common ETHOS runtime, attestations, receipts, and release records | Active governance runtime or durable evidence with current consumers | Preserve; lifecycle and retention policy belong to ETHOS, not AIGW.           |
| Signed release tags and OpenSpec archives                             | Immutable product chronology and release evidence                    | Preserve until a separate authorized retention policy proves them disposable. |

No tracked `evidence`, `claims`, `chronicle`, `.code-memory`, or `.ethos/state`
container exists. The repository currently exposes one work lane, one candidate
checkout, and no remote proposal branch. Further deletion remains scoped to the
semantic closure that proves a specific implementation, test, configuration,
or document has no consumer.

## Migration Plan

1. Freeze and inventory the current accepted product, published bytes, user journeys, tracked carriers, dependencies, and residue.
2. Repair and verify each product journey while retaining AIGW 0.1.0 as the working baseline.
3. Reorganize source and tests around the verified semantic owners; delete superseded material in the same closure.
4. Consolidate quality and CI projections, then upgrade direct dependencies under the complete graph.
5. Rewrite current documentation from the accepted design and verify navigation and rendering.
6. Run exact-HEAD, native, published-artifact, installation, performance, and residue acceptance; publish only a new version when product bytes change.
7. Archive this Change and retire its proposal and Work Lane through ETHOS after every task is evidenced.
