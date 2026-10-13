# Tasks

[Acceptance routing](design.md#requirement-and-acceptance-routing) owns requirements and commands; the [Migration Plan](design.md#migration-plan) owns post-archive delivery. This is the only progress checklist. Git, CI and source-bound native evidence retain execution history.

## 1. Authority and Baseline

- [x] 1.1 Reconcile the signed base and in-flight Changes by semantic owner; verify exact refs, leases and overlap before integration or retirement.
- [x] 1.2 Map each remaining user-visible obligation to one OpenSpec requirement, implementation owner and native acceptance command; remove duplicated status.
- [x] 1.3 Capture the protected predecessor, selected bindings, backend, original callers and rollback bytes through metadata-only inspection before host cutover.
- [x] 1.4 Reproduce installed and source no-Token/no-client setup-to-sync failure in isolated state; retain a distinguishing RED regression and prove no host writes or residue.

## 2. One Onboarding and Readiness Decision

- [x] 2.1 Unify activation prerequisites for capability, selection, credential, projection and verification; reject next actions with missing preconditions.
- [x] 2.2 Consume that decision in manifest and guided setup; prove equivalent Account choices without credentials and no premature no-op sync recommendation.
- [x] 2.3 Consume the same decision in sync preview, status, check and doctor; prove equivalent human/JSON states and next actions on identical inputs.
- [x] 2.4 Verify explicit `use --for` preserves other clients and lets `check` pass without a global default or `--all` through the existing CLI regression.
- [x] 2.5 Exercise Token-before-client, client-before-Token, both-absent and one-of-many-Accounts through `manifests/team.toml`; prove late sync without irrelevant credentials.
- [x] 2.6 Reconcile projections semantically; prove byte-preservation of unrelated edits, precise conflicts and compensation after each owned-write failure.
- [x] 2.7 Admit each selected Account/client through endpoint and inference checks; prove prompt-free authorization classification and no observation or charging of unrelated Accounts by `check --for`.

## 3. Credential Reader Succession

- [x] 3.1 Reproduce fixed-path succession failure without deleting active bytes; retain a lifecycle regression distinguishing predecessor and successor.
- [x] 3.2 Compare fixed copies, stable indirection and versioned paths against a no-new-entity baseline on three OSes; record one qualified choice in DR-0011.
- [x] 3.3 Implement that choice in credential/sync owners; prove original commands, exact identity, permissions/ACLs, interruption, conflicts and no Token disclosure.
- [x] 3.4 Qualify rollback, uninstall and exact cleanup; preserve cached, explicit, unknown and rollback consumers without prefix/age-based deletion.
- [x] 3.5 Qualify published-predecessor reader succession, rotation, rollback, forward and cleanup on three OSes with isolated native items or explicit ENV; preserve original commands and disclose cached public-link risk. Operator-item and package-link acceptance depend on 5.4 and the Migration Plan.
- [x] 3.6 Preserve explicit external credential commands outside AIGW ownership; prove exact paths, live-client verification boundaries and deletion of only unused AIGW-owned readers.

## 4. Provider, Model, and Client Extension

- [x] 4.1 Remove parallel inference and Provider-name branching from Account/Model/Route/protocol/capability/recommendation owners; prove synthetic Provider admission.
- [x] 4.2 Qualify the shipped DMXAPI/UCloud/AIHubMix catalogue against current IDs and bounded live inference; retain consistent names and dated evidence for each model/variant.
- [x] 4.3 Prove recommendations select only usable unselected Routes, preserve explicit choices and exclude manual-only Providers without claiming request-time failover.
- [ ] 4.4 Qualify selected non-OpenAI-family Responses models in Codex's actual chooser, authentication and tool loop; resolve metadata admission or report native limitations without forged model lists.
- [ ] 4.5 Requalify Claude Code, Codex and Hermes independently for native protocol, selection, credentials, retained continuation and rollback with real Provider credentials; prove verification isolation through the [native-client contract](design.md#3-qualify-a-client-adapter-do-not-infer-support-from-a-file). Claude Desktop reads Tokens only through its installed projection, so its Chat, Cowork and Code requalification with the 0.4.0 reader follows the installed cutover under the [Migration Plan](design.md#migration-plan).
- [x] 4.6 Evaluate pi, OpenCode, WorkBuddy, Qoder and other proposed clients; admit an Adapter only with executable, projection, ownership, withdrawal and real tool-loop evidence.
- [x] 4.7 Qualify optional client-scoped forwarding through selection, resolution, credentials, dry-run, projection and rollback; preserve upstream/provider identity and direct operation, prove retained-child tools and exact cleanup, and leave Proxy transport/lifecycle outside AIGW.

## 5. Portable Native Product Lifecycle

- [x] 5.1 Reconstruct a fresh current-HEAD lane's locked Go/Node/npm/tool environment; prove empty-cache bootstrap, independent mutable state, ambient-fallback rejection and exact cleanup.
- [x] 5.2 Qualify Linux container/native setup, selection, projection, update, rollback, uninstall and cleanup with exact release bytes and retained predecessor state.
- [x] 5.3 Qualify Windows native lifecycle, ACLs, executable replacement, quoted paths, noninteractive ENV and installed Codex/Claude; identify each unproved client mode.
- [ ] 5.4 Qualify isolated macOS Keychain and ENV lifecycle plus signed retained-item authorization under the approved `macos-keychain-item-migration`; preserve prompt-free original-caller continuity. The installed package-manager upgrade, rollback and forward follow under the [Migration Plan](design.md#migration-plan).
- [x] 5.5 Qualify missing-client setup and late Codex/Claude installation on each OS; sync only admitted installed clients and preserve user files.
- [x] 5.6 Prove exact owned-resource cleanup after success, failure, timeout and interruption; preserve installations, client state, rollback and evidence.

## 6. Quality, Supply Chain, and Performance

- [ ] 6.1 Update direct Go/npm/OpenSpec/Mise/release tools to latest stable compatible pins and locks; prove clean-context reproducibility, native Windows acquisition, sibling-network containment and license/security observation.
- [x] 6.2 Apply one native Go format/lint/static/test/race/coverage policy to product, tools and tests; remove duplicated rules and reject unowned warnings.
- [x] 6.3 Simplify semantic hotspots and enforce risk-justified ELOC, statement, nesting and complexity bounds without mechanical splitting or suppression.
- [x] 6.4 Qualify Markdown, Mermaid rendering, links, TOML/YAML/JSON/CUE, shell and generated text on tracked inputs; malformed or unreachable authored content must fail its owning gate.
- [x] 6.5 Qualify locked-candidate dependency hygiene, dead code, secrets, SBOM, licenses, checksums, signatures and provenance; disclose nonblocking vulnerabilities and delete unconsumed duplicate scans.
- [ ] 6.6 Qualify startup/setup/sync/credential/projection/build/CI against retained-predecessor budgets through the [native measurement contract](../../../docs/governance/change-and-release-policy.md#behavioral-and-quantitative-evidence); preserve raw samples, host/order and exact identities, complete Windows/macOS Keychain/final-signing evidence, and optimize only diagnosed owners.
- [x] 6.7 Reject warnings and malformed public errors at their origin; prove precise human/JSON states and safe actions without private paths, secrets or implementation leakage.

## 7. CI and Dual-Peer Admission

- [x] 7.1 Qualify one CUE graph and its GitHub/GitLab projections; reject drift or missing source/native/release/publication ownership.
- [x] 7.2 Qualify exact-SHA event/check mappings and candidate proposals on both peers; prove proposal/review/maintainer/dev/main/tag bindings without treating projections or manual runs as post-archive publication.
- [x] 7.3 Separate independent quality/bootstrap/native/client/release work and share only immutable evidence or content-addressed caches; cancel superseded review before noninterruptible jobs.
- [ ] 7.4 Qualify effective status enforcement, unprotected proposals, guarded merges, signer trust, source-ref auto-delete and untrusted-runner isolation/cleanup on GitHub; `gitlab-peer-reconciliation` owns GitLab's admission. Actual dev integration/checks/ref deletion and archived main/tag effects remain required by the [Migration Plan](design.md#migration-plan), not this prerequisite.
- [x] 7.5 Qualify one-peer and offline-local operation; preserve independent local/peer progress and peer-specific parity claims during another peer's outage.

## 8. Repository Topology, Documentation, and Deletion

- [x] 8.1 Consolidate product/tool/test semantic owners and native configuration/sidecars; prove isolated discovery/bootstrap, symmetric CI, tagged predecessors and exact root retirement without facades.
- [x] 8.2 Reconcile tracked entry, architecture, decision, operations, contribution and release docs; prove linked English navigation and no untracked prerequisites.
- [ ] 8.3 Qualify CLI/document presentation, Markdown spacing and references through 8.3.1; verify native output and rendered layouts. `gitlab-peer-reconciliation` owns ETHOS identity/reference admission and historical peer tag reconciliation.
  - [x] 8.3.1 Correct Changelog headings, peer history and native locators; preserve historical notes and distinguish prepared release links from published tags.
- [x] 8.4 Compare mature gateway/config/client/release capabilities with AIGW differentiators; record source-backed adopt/reject decisions and delete replaced owners.
- [x] 8.5 Inventory branches, outputs, records, tags, caches and lanes by owner/consumer; retire proved-disposable pre-archive residue and bind retained evidence/rollback/client consumers to explicit retirement triggers.
- [x] 8.6 Migrate the Client Projection Edition Provider to Publisher v2; prove exact packages, preserved authored meaning/media, direct/declarative parity, offline relocation, refusal and Git rollback before deleting superseded v1 owners.

## 9. Frozen Source and Pre-Archive Acceptance

- [ ] 9.1 Audit every changed requirement against source, tests, CLI, team manifest, quality/CI and docs; remove contradictions and unused compatibility paths before freezing inputs.
- [ ] 9.2 Qualify focused RED/GREEN, native static/behavior gates and strict official OpenSpec on the frozen source; no skipped required gate or warning counts as pass.
- [ ] 9.3 Qualify one signed untagged three-OS candidate's exact-byte install, predecessor upgrade/rollback, credentials and real clients; bind hashes/provenance and disclose missing evidence. Publication, Homebrew cutover and final retirement follow the Migration Plan; maintenance 0.3.5 does not qualify final 0.4.0.
