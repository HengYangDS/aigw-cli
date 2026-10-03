# Tasks

Requirements and verification commands are owned by the
[acceptance routing](design.md#requirement-and-acceptance-routing); the
[Migration Plan](design.md#migration-plan) owns post-archive delivery.
This checklist is the only progress ledger. Historical execution text remains
in Git; original raw results stay with their source-bound verification owner.

## 1. Authority and Baseline

- [x] 1.1 Reconcile the signed accepted base, the staged Hermes archive, and
      every in-flight AIGW Change by semantic owner; verify exact refs, lease
      holders, and overlap before absorbing or deleting any lane.
- [x] 1.2 Map each remaining user-visible obligation to its canonical OpenSpec
      requirement, implementation owner, and native acceptance command; remove
      duplicate status prose instead of creating another ledger.
- [x] 1.3 Capture protected 0.3.1 installation, selected Client Bindings,
      credential backend, running callers, and rollback bytes without reading or
      logging Token values; verify metadata before any host cutover.
- [x] 1.4 Reproduce the installed and current-source no-Token/no-client
      setup-to-sync contradiction in an isolated home, including cleanup and no host
      writes; retain one focused RED CLI regression.

## 2. One Onboarding and Readiness Decision

- [x] 2.1 Make the existing activation owner return capability, selection,
      credential, projection, and verification prerequisites separately; focused
      tests reject a next action whose precondition is absent.
- [x] 2.2 Consume that decision in manifest setup and guided setup; test that
      zero credentials yield equal compatible Account choices and never recommend
      immediate no-op sync.
- [x] 2.3 Consume the same decision in sync preview, status, check, and doctor;
      assert equivalent human/JSON states and next actions on identical input.
- [x] 2.4 Verify the accepted `aigw use --for` path preserves every other
      client's explicit binding and `aigw check` succeeds without a global
      default or `--all`; reuse the existing CLI regression rather than
      reimplementing selection.
- [x] 2.5 Exercise Token-before-client, client-before-Token, both-absent, and
      one-of-many-Accounts journeys through the shipped `manifests/team.toml`; prove
      late `sync` and no irrelevant credential requirement.
- [x] 2.6 Reconcile managed projections semantically, preserving unrelated edits
      byte-for-byte and reporting exact ownership conflicts; inject failure after
      each owned write and verify compensation.
      Current Hermes catalogue compensation, no-op preservation and selected-model
      drift refusal pass without a new adapter implementation: receipt
      `actual-hermes-catalogue-compensation-20261002/delivery.json` (`401a202d`).
- [x] 2.7 Admit a selected live Account and client through endpoint and
      inference checks, while classifying absent native authorization without
      prompts or backend fallback; keep supplier-specific failures scoped to that
      Account. A focused `check --for` must not observe or charge another
      enabled client's Account.
      Three selected Sol 6.1 Account probes pass with one attempt each; receipts
      `1eef7dd0-selected-live-provider-20261003/delivery.json` preserve explicit
      host choices. Actual client and signed-byte acceptance remain in 4.5/9.3.

## 3. Credential Reader Succession

- [x] 3.1 Reproduce the fixed-path reader's inability to consume a later signed
      version without deleting its active bytes; contrast old and successor behavior
      with a failing lifecycle test.
- [x] 3.2 Compare the current AIGW copy, stable indirection, and direct versioned
      paths with a no-new-entity baseline on macOS, Linux, and Windows; amend
      DR-0011 with one qualified choice before implementing a cutover.
- [x] 3.3 Implement only the selected strategy in the existing credential and
      synchronization owners; test original commands, exact identity, private
      permissions or ACLs, partial conflict, interruption, and no Token disclosure.
- [x] 3.4 Exercise rollback, uninstall, and exact owned-byte cleanup for that
      strategy; preserve unknown, cached, explicit, or rollback consumers and
      reject prefix/age-based deletion.
- [ ] 3.5 Run published-predecessor-to-successor reader journeys on macOS,
      Linux, and Windows with actual native stores or the explicit environment
      backend and isolated synthetic native items before archive. Verify stable
      native reader identity across signed successors without treating signing
      as item authorization. Preserve original versioned commands and disclose
      the cached 0.3.1 public-link risk. Final operator-item authorization and
      the real package-link transition belong to the post-archive
      [Migration Plan](design.md#migration-plan), not this pre-archive checkbox.
      Historical synthetic Keychain/Credential Manager scope remains in 5.3-5.4
      and `build/verification/4dd8504a411135674ad72b0424bb10803882a62c/proposal-publication/`;
      current Debian Secret Service succession and visible registration warnings
      remain in 5.2. Neither proves durable login or warning-free native use.
      Remaining: final signed macOS reader's stable designated requirement;
      the old `f4cd7c51` receipt cannot qualify a different candidate.
- [x] 3.6 Keep an explicitly configured external credential command outside AIGW
      ownership; test its exact-path preservation, live-client verification
      boundary, and removal of only unused AIGW-owned readers.

## 4. Provider, Model, and Client Extension

- [x] 4.1 Audit Account, Model, Route, protocol, capability, and recommendation
      declarations for parallel inference or Provider-name branches; delete the
      duplicate owner and prove synthetic Provider admission.
- [x] 4.2 Review the shipped catalogue against current upstream IDs and bounded
      live inference for DMXAPI, UCloud, and AIHubMix; retain only qualified
      models/variants and one consistent naming grammar, with source and date for
      each claim.
      The dated 26-Model/57-Route manifest and all qualification/withdrawal
      decisions are owned by [catalogue qualification](../../../docs/research/provider-model-qualification.md#provider-catalogue-and-route-evidence).
      Original inference, later plain DMXAPI continuation, and intermittent
      failures keep their distinct dates; catalogue reuse is not live availability.
- [x] 4.3 Prove that ordered existing recommendations select only usable
      unselected Routes, preserve explicit choices, and exclude manual-only
      Providers; no control-plane command claims request-time failover.
      DMXAPI is primary, UCloud the only automatic alternative and AIHubMix
      manual-only. Shipped-team and config-copy acceptance preserve all four
      explicit bindings; `current-manifest-a210-profile-acceptance-20261003/`
      retains byte-exact rollback/no-op and rejected repeat-retirement controls.
- [x] 4.4 Qualify Codex with selected non-OpenAI-family Responses-compatible
      models using its actual model chooser, authentication and tool loop; state
      native limitations instead of forging a model list.
- [ ] 4.5 Requalify Claude Code, Claude Desktop, Codex, and Hermes independently
      for native protocol, model selection, credential and rollback behavior; do not
      infer Desktop from CLI or endpoint reachability from real-client success.
      Current macOS Codex 0.160.0/Claude Code 2.1.288 authentic-0.3.1 four-stage
      environment journeys and read-only shell use pass; exact bytes, state and
      cleanup remain in `13e6525c/macos-latest-client-input-20261004/`. Fourteen
      general Routes retain incomplete metadata qualification where observed.
      Current Claude 2.1.288 passes public setup/use/check/verify, Read tools and
      same-session recall on three Sonnet 5.5 Routes (DMXAPI CC) and UCloud Opus
      5.5. Current Codex program/team/client bytes match retained AIHubMix and
      UCloud Sol 6.1 continuations; the missing DMXAPI CDX cell passes in 31.33s.
      Full current-client evidence and exact cleanup are bound in
      `13e6525c/current-upstream-review-20261004/`; earlier plain DMXAPI timeout
      and native invocation failures remain separate. The timeout's cause is
      unproved; the redundant enable invocation was diagnosed and corrected.
      Windows latest-client four-stage/Route results pass, but its service-
      Session 0 Codex `pwsh` loop fails `0xC0000142`. Exact upstream 0.160 code
      and issue #46412 support Fleet's WinSta0 hypothesis, not actual-child root
      proof. Keep private-desktop isolation and qualify the original loop in a
      supported native context or with a vendor fix. `AIGW13e-Windows-*` retains
      raw failure, eight pause states, VM isolation and exact cleanup.
      Latest Linux Claude/Hermes scope is in 5.2; historical Windows is in 5.3.
      Hermes tools/history retain original bytes; Claude Desktop Chat/Cowork/
      Code remains macOS-only. Final signed/store qualification stays in 9.3;
      operator/GUI/Homebrew cutover follows the [Migration Plan](design.md#migration-plan).
      Claude-in-Codex still needs external Responses tool/replay qualification.
- [x] 4.6 Evaluate pi, OpenCode, WorkBuddy, Qoder, and other proposed agents
      against the same adapter contract; implement only adapters whose executable,
      projection, ownership, withdrawal and real tool loop can all be proved.
- [x] 4.7 Verify direct HTTPS, optional external Responses endpoint, and
      no-Proxy/no-Forge/no-client operation through the same Account path; assert
      AIGW does not install, route traffic through, or manage the external service.

## 5. Portable Native Product Lifecycle

- [x] 5.1 Reconstruct a fresh current-HEAD Git worktree's locked Go, Node, npm
      and tool environment with independent mutable state; test empty HOME/cache
      bootstrap, reject ambient fallback, and remove exact owned test state.
- [ ] 5.2 Run Linux container and native-host setup, selected provider,
      projection, update, rollback, uninstall, and cleanup journeys using exact
      release bytes and one retained predecessor state.
      Source `13e6525c` passes authentic-0.3.1 Secret Service succession and the
      original five-case resource suite on native Debian and in a container,
      including the actual one-minute deadline. Two native registration warnings
      remain, not durable-login/warning-free proof. Native Codex 0.160.0 passes
      retained-predecessor stages and sandbox tool use; container Claude 2.1.286
      and Hermes 0.21.5 pass their scopes. Latest native Claude 2.1.288 now passes
      all four retained-predecessor stages in 2.35s; official Hermes 0.21.5 passes
      its unchanged four stages, file tools and continued history in 100.54s.
      Fleet's `AIGW13e-Linux-*` terminal receipts retain raw results, all nine
      inputs, 15,075 official source hashes/original modes and exact UID 997,
      account/service/test cleanup with all fifteen pause states/VM isolation
      restored. Stage-mode failures remain retained; no vendor patch or passed-
      group rerun was used. Current Codex 0.160.0 container four-stage lifecycle
      and native read-only shell loop also pass in 71.64s using the same exact
      `13e6525c` candidate/predecessor and original tester. Existing namespace
      profile, cap-drop, no-new-privileges, network denial and native NSS identity
      remain explicit in `13e6525c/container-codex-current-20261004/`; exact
      container/scratch retirement and foreign-resource preservation pass. The
      tool-loop metadata warning still prevents full Route qualification; it is
      not suppressed. Earlier results retain their own bytes/backends. Remaining:
      final signed artifact/client acceptance in 9.3. UID 1003/Runner 105 is not
      neutral; private DBus alone does not establish store isolation.
- [x] 5.3 Run equivalent Windows native journeys, including ACL, executable
      replacement, path quoting, noninteractive environment Token and installed
      Codex/Claude consumers; report each unproved client mode explicitly.
      Run 37123751273/job 111204828386 passes authentic 0.3.1 (`54b7fb16`) to
      candidate (`b4f31bfe`) Credential Manager succession, twelve client stages,
      fourteen selections and native file tools. Observed clients are Codex
      0.159.3, Claude 2.1.286, Hermes `f97608f`/0.21.5; unique safe output is in
      `4dd8504a/proposal-publication/github-windows-*`. Final archive/provenance
      and consumer hashes are not independently replayed. Earlier ARM64 scope is
      retained; latest-client failure, final bytes, security and Runner containment
      remain in 4.5, 9.3, 6.5 and 7.4.
- [ ] 5.4 Run equivalent macOS journeys with native Keychain authorization and
      environment backend separately in isolated acceptance; final operator-item
      authorization remains required before post-archive installed cutover.
      No password/biometric retry loop, service restart, or hidden native-store
      policy change is permitted.
      Intel macOS run 37122724491/job 111201877396 passes authentic 0.3.1
      (`ffc89cb9`) to candidate (`7b2cb1fa`) synthetic Keychain succession,
      retained readers, rotation, rollback, re-upgrade and cleanup; unique markers
      remain in `4dd8504a/proposal-publication/github-keychain-peer-*`. Separate
      environment-backend lifecycle is in 4.5. Remaining: final native-store bytes
      and stable Developer ID identity; neither result proves operator-item access
      or Homebrew replacement.
- [x] 5.5 Verify missing Codex/Claude at setup and later installation on each
      platform; deferred sync must touch only installed admitted clients and
      preserve user-owned files.
      Original cross-platform deferred-installation subtests pass without skips
      in run 36560334613, including Desktop only on macOS/Windows. Actual-client
      and final published-byte claims remain independently owned.
- [x] 5.6 Compare owned process, helper, temporary, journal, build and
      client-projection resources before/after success, failure, timeout and
      interruption; exact teardown preserves active installations and evidence.
      Tracked success, failure, parent-exit, interrupt and actual 60-second
      deadline cases prove exact owned process/installation cleanup; platform
      results and raw failures retain their original candidate identities.

## 6. Quality, Supply Chain, and Performance

- [x] 6.1 Audit every direct Go, npm, OpenSpec, Mise and release-tool version
      against the latest stable compatible upstream; update authored pins and locks
      once, then prove clean-context reproducibility and license/security
      admissibility.
      Latest stable-compatible authored Go/npm/Mise/OpenSpec pins, complete native
      locks and registry signatures pass. Native npm resolves the same graph;
      stable parents constrain three newer transitive suggestions, and Go's newer
      consumed suggestion is untagged development. Retain parents' pins: neither
      transitive compatibility violations nor vendor patches are admitted.
      Current upstream evidence remains in `13e6525c/current-upstream-review-20261004/`;
      latestness does not close security 6.5.
- [x] 6.2 Make Go format, lint, static analysis, test, race and coverage rules
      cover product, tools and tests with zero unowned warnings; remove duplicate
      custom rules when a mature native tool proves the same property.
      Signed `4dd8504a` passes both peers' native Go gates with zero issues and all
      canonical packages observed: GitHub macOS coverage 13,752/14,454 (95.14%),
      GitLab Linux 13,634/14,341 (95.07%). Unique safe summaries remain in
      `4dd8504a/proposal-publication/`; raw hosted profiles were not exported.
      Diagnostic-tail RED/GREEN, exact cleanup, original two-minute cache budget
      and independent six-target archive test remain accepted; security is in 6.5.
- [x] 6.3 Measure ELOC, logical statements, nesting and complexity by semantic
      owner; set risk-justified blocking bounds and simplify real hotspots without
      mechanical file splitting or suppressions.
      Cyclop remains 25. Native macOS/Linux/Windows trials at 20 produce
      70/70/71 findings and at 15 produce 235/231/231; semantic review found no
      justified blanket reduction. The case is closed without suppressed rules,
      forwarding helpers or mechanical splits; other bounds remain blocking.
- [x] 6.4 Exercise Markdown, Mermaid rendering, internal/external links, TOML,
      YAML, JSON, CUE, shell and generated-text checks on tracked content; a
      malformed or unreachable authored carrier must fail the relevant gate.
      Tracked text/configuration gates, native Markdown/Mermaid behavior and
      malformed-input controls pass. Windows catalogue assertions parse actual
      TOML semantics instead of comparing renderer-specific escape spelling.
- [ ] 6.5 Verify dependency hygiene, dead-code, secrets, SBOM, vulnerabilities,
      licenses, checksums, signatures and provenance from the exact locked
      candidate; delete unconsumed parallel scanners or reports.
      Current native Renovate observes CUE Codex 0.160.0/Claude 2.1.288 pins without
      relaxing release-age/automated proposal policy. Syft 1.54.0 six-platform
      provenance locks and real SBOM tests pass; source receipt is in
      `13e6525c/current-upstream-review-20261004/`. Final native client scope is in
      4.5/5.2; SBOM/license/signature/provenance retain their original candidates.
      Blocking: supported OpenSpec/Mermaid still consume `braces` 3.0.3,
      `GHSA-vfj7-8cjw-p6xm` (High 8.7); both quality jobs fail, and no supported
      patched stable chain is proved. No ignore, downgrade, fork/vendor patch,
      compatibility violation or schema restriction is admitted. Removing the
      Markdown CLI intermediary deleted seventeen packages, not this chain.
      The release owner refuses failed/malformed/incomplete dependency evidence
      before build/SBOM/signing; source rules and construction race tests pass.
      Its real locked `b2dbeeb7` refusal takes 7.06s and preserves raw failure,
      installed/retained bytes and exact workspace absence. This refusal is not
      bypassed to manufacture signed assets; final exact-lock matrix stays open.
- [ ] 6.6 Measure startup, setup, sync, credential read, native projection,
      build and CI costs against predecessor budgets on a quiet host; preserve raw
      samples and optimize only diagnosed owners.
      Original Hyperfine 1.20.0 compares the current macOS candidate with authentic
      0.3.1: 32 blocks/1,280 samples, five warmups, reversed order, no warnings and
      calibrated memory pass in 40.96s; pooled p95 helper 14.30ms, projection
      49.14ms, setup 43.18ms, sync 30.99ms. Raw data remains in
      `13e6525c/performance-current-env-20261004/`.
      Linux environment/file measurement on a native disk volume completes 48
      blocks/1,920 successful samples in 35.71s. Candidate absolute budgets pass,
      but four statistical warnings keep the original gate inconclusive. The
      same operation captures every raw JSON/log and summary before exact
      cleanup in `13e6525c/performance-linux-disk-output-20261004/`; raw capture
      is repaired without changing thresholds or attributing an unproved cause.
      Earlier tmpfs failure and incomplete raw capture remain retained in
      `13e6525c/performance-linux-env-file-20261004/`. Dedicated quiet-host,
      native-store, final signed-byte, inference and build/CI qualification remain
      open; another full measurement requires genuinely qualified new inputs.
- [x] 6.7 Assert warnings and malformed public errors fail at their origin;
      human and JSON output must expose precise state and safe action without
      private paths, Token fragments, internal modules or tracebacks.
      Selected `25006e50` Grok refusal, clean Sol verification, nonempty
      checkpoint preservation and Codex/Claude retained-state regressions pass
      (`27e761ad`, `faa46eab`). Native capture rejects explicit diagnostics while
      preserving normal progress; original failed controls remain retained.

## 7. CI and Dual-Peer Admission

- [x] 7.1 Reconcile the complete CUE CI graph with generated GitHub and GitLab
      projections; prove no hand-edited workflow drift or missing source, native,
      release or publication owner.
- [ ] 7.2 Prove both peers' exact-SHA event-to-check contract for developer
      proposal create/update/review, maintainer fast-forward, accepted `dev`/`main`,
      and signed-tag pushes. Run the exact candidate's proposal checks on both
      peers; CUE and projection regressions must bind each other event to its
      intended commit SHA and required-job set. Actual `main`/tag results against
      the archived SHA are post-archive release acceptance under Migration Plan;
      do not infer them from projection tests or a manual run.
      Signed `4dd8504a` proposal checks (GitLab MR !178/pipeline 9400; GitHub
      PR #162/run 37120920504) pass macOS/Linux/Windows/Secret Service, while
      quality fails security 6.5. Intel job 111196700148 passes in 806s with cold
      budget unchanged. All watchers are terminal; historical `82f66fe9` timeout
      retains unproved cause. Final exact-source required checks, accepted events
      and guarded integration remain open; manual runs do not replace them.
- [x] 7.3 Separate fast quality, locked bootstrap, macOS, Linux, Windows, native
      clients and release construction where independence saves time; share only
      immutable evidence and content-addressed caches. Cancel superseded review
      work only before a noninterruptible native Shell or release job starts.
      One CUE graph and one release parser own ordinary source versus explicit
      artifact/client/performance dispatch. No Forge credential reaches native
      journey children. Final cold-peer and persistent-runner acceptance remain
      in 7.4/7.5; shipped-client bytes remain in 9.3.
- [ ] 7.4 Prove required-status enforcement, unprotected proposal branches,
      guarded-merge policy, source-branch auto-delete configuration, signer trust
      and branch protection on each selected peer without interactive
      authentication or divergent commit
      identities. Prove untrusted review code cannot observe persistent Shell
      runner credentials or protected-job state; retain required native evidence.
      A policy readback proves configuration, not deletion: observe the exact
      proposal source ref absent after its guarded merge in the delivery sequence.
      Required check names, strict/admin enforcement, GitLab merge policy and
      unprotected proposals are observed; configuration is not enforcement proof.
      CUE separates protected/unprotected Linux registrations. Remaining:
      untrusted Shell credential containment, actual protected-event execution,
      guarded merge and exact source-branch deletion on each selected peer.
- [ ] 7.5 Test one-peer-only and offline-local operation; an unavailable GitLab
      or GitHub must not turn the other selected peer or the local product into a
      hidden dependency, and parity claims remain peer-specific.
      Current networking-denied native install/setup/status/doctor/catalog/preview/
      export/uninstall passes with 57 Routes/3 Accounts preserved and absent
      credentials/clients deferred; independent exact readback/cleanup is in
      `13e6525c/offline-current-native-product-20261004/`. Earlier sole-peer local
      Git controls keep their candidate. Lock coverage is identity, not availability.
      Run 37122724491 proves cold Intel macOS URL substitution/locked glab.
      Full platform/tool graphs, upstream-outage denial and real GitLab Job Token
      downloads remain unproved; this does not close canonical CI.

## 8. Repository Topology, Documentation, and Deletion

- [x] 8.1 Audit `src`-equivalent Go packages, `internal/`, `cmd/`, `tools/`,
      tests, root and `.config` by semantic responsibility; replace suffix-flat or
      mixed owners with cohesive packages and remove forwarding facades.
- [x] 8.2 Reconcile tracked README, architecture, decision, operations,
      contributing and release documentation with current product behavior; all
      canonical pages must be linked, English, navigable and free of references to
      untracked prerequisites.
      All 23 canonical pages are reachable from the tracked documentation root;
      native metadata, formatting, links, spelling and semantic checks pass.
      A supplemental count-only README demand is not adopted as a second rule;
      the existing product/quality owners remain authoritative.
- [ ] 8.3 Close the document presentation and reference review through 8.3.1
      and 8.3.2. The layout/whitespace review has passed; installed ETHOS reference
      qualification and historical peer identity reconciliation remain open.
      Root help aligns at display column 34 for native/renamed commands across
      widths/color modes. One Markdown whitespace owner rejects the former
      false-green formatter and covers wrapped/nested/quoted items, nested
      fences/tables and literal-code preservation; immutable archives stay intact.
  - [x] 8.3.1 Correct neutral Changelog headings, explicit peer history, and native repository locators without changing existing historical notes; verify strict release metadata and each actual peer destination. Keep the prepared release links distinct from unpublished tags.
        All 60 headings and 120 explicit peer links preserve historical bodies;
        referenced tags exist on their selected peer. Local-only v0.3.2 remains
        unlisted and prepared v0.3.3 does not claim publication. Historical object
        identity and installed prevention remain in 8.3.2.
  - [ ] 8.3.2 Qualify the accepted installed ETHOS identity/reference contract, reject reachable wrong-repository destinations, and reconcile the audited historical peer tag identities through authorized native repair without a private map or copied checker.
- [x] 8.4 Compare mature gateway, config, client and release libraries with
      retained AIGW differentiators; record one source-backed adopt/reject decision
      per candidate and delete any replaced hand-written owner.
- [x] 8.5 Complete the pre-archive inventory of branches, outputs, records,
      tags, caches and worktrees by exact owner and consumer; retire currently
      disposable residue and identify retained consumers and retirement triggers.
      Preserve immutable evidence, failed receipts, rollback and running clients.
      Post-archive release and final-lane retirement are required by the
      [Migration Plan](design.md#migration-plan), not prerequisites for this task.
      `13e6525c/housekeeping/` names every retained owner/consumer/retirement trigger.
      Independent readback verifies removal of eighteen obsolete candidate archives
      (87,901,974 bytes) and two GitLab mirror packages (122,606,962 referenced
      bytes), not server-GC recovery. Lock mirrors/native inputs/installed readers/
      rollback/failed receipts/metadata stay intact. Proposal withdrawal needs
      guarded integration; final tags, release outputs and lane retirement follow
      their consumer transitions. Foreign/unproved history/recovery is preserved.
- [x] 8.6 Migrate the Client Projection Edition Provider to the published
      Publisher v2 contract. Preserve the authored Claim Model, four reader
      questions, independent media and AIGW acceptance authority; prove exact
      package inputs, direct/declarative equivalence, relocated offline replay,
      invalid-input refusal and Git-bound rollback before deleting the v1
      materializer, captured source copies and duplicate generated identities.
      Exact Publisher v2 alpha.7 inputs pass installed direct/declarative
      equivalence, relocated replay, tamper refusal and Git-bound rollback.
      Twenty-one superseded files are removed, over 7,000 tracked lines deleted,
      and source/text/native rendering checks pass without changing authored
      claims. This is source integration, not another product's publication.

## 9. Frozen Source and Pre-Archive Acceptance

- [x] 9.1 Review each changed requirement against source, tests, CLI, team
      manifest, quality/CI projection and docs; remove contradictory old text and
      unused compatibility paths before freezing inputs.
      Seven official delta merges preserve inherited scenarios while replacing
      contradictory fallback requirements; focused failure-phase controls pass.
      Final native/candidate, security and publication remain separate decisions.
- [x] 9.2 Run focused RED/GREEN suites, native static/behavior checks and strict
      official OpenSpec validation with pristine output on the frozen source; no
      skipped required gate or warning counts as pass.
      Focused native/static/text/OpenSpec checks and distinguishing counterexamples
      pass in their declared scopes. Exact-source native results are in 6.2/7.2;
      final full security proof remains blocked by 6.5, not silently skipped.
- [ ] 9.3 Qualify one signed untagged macOS/Linux/Windows candidate with
      exact-byte install, predecessor upgrade/rollback, credentials and real-client
      journeys; retain the source and artifact hashes and disclose missing platform
      evidence.
      Native/client evidence is owned by 3.5, 4.5 and 5.2-5.4; exact security and
      provenance by 6.5, performance by 6.6, peer enforcement by 7.2-7.5. Each
      retains its candidate identity. Remaining: one signed exact-source matrix
      and byte-bound lifecycle, stable reader identity, native stores, clients,
      security, provenance and performance. Production signing/publication/
      installed Homebrew cutover follows the [Migration Plan](design.md#migration-plan).
      Equal product inputs do not rebind signatures, source epoch or provenance.
