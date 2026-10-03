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
      Current-source synthetic macOS Keychain and Windows Credential Manager
      succession pass at `4dd8504a`; exact native markers are in
      `build/verification/4dd8504a411135674ad72b0424bb10803882a62c/proposal-publication/`.
      Remaining: current isolated native Linux succession and the final signed
      macOS reader's stable designated requirement. The old `f4cd7c51` identity
      receipt proves its own bytes, not the selected `25006e50` ad-hoc program.
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
      Selected `25006e50` macOS bytes pass Codex/Claude retained-0.3.1 lifecycle
      (`faa46eab`), official Hermes file tools/history and four-stage lifecycle
      (`9cbfa591`), and native Codex Sol 6.1 tools/continuation on all three
      Providers (`5404b5f4`, `534a3a75`, `b0a14a83`, `0c4bc791`). The earlier
      DMXAPI timeout remains historical with unproved cause. Unchanged Claude
      Desktop Chat/Cowork/Code is qualified only on its declared macOS surface.
      Current-source Windows Codex/Claude/Hermes four-stage native acceptance
      passes in run 37123751273; evidence is in 5.3. Current Linux and final
      signed-candidate client cells remain open in 5.2/9.3. Native-store identity,
      supplier security and performance keep their separate owners. Actual
      operator authorization, installed GUI and Homebrew cutover remain
      post-archive [Migration Plan](design.md#migration-plan) obligations.
      Claude-in-Codex is not admitted by an Anthropic-only Route; a real
      external Responses tool/replay path is required, not model-list invention.
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
      Earlier container and native-host results retain their own candidate and
      backend identities; no historical package is relabelled current. Remaining:
      current-source native Linux Secret Service under a genuinely isolated
      identity, current Codex/Claude/Hermes retained-predecessor journeys, and
      final signed-byte acceptance. UID 1003 owns Runner 105 and is not neutral;
      a private DBus alone does not prove isolation from that Runner's stores.
- [x] 5.3 Run equivalent Windows native journeys, including ACL, executable
      replacement, path quoting, noninteractive environment Token and installed
      Codex/Claude consumers; report each unproved client mode explicitly.
      Current-source Windows run 37123751273 / job 111204828386 passes authentic
      published 0.3.1 (`54b7fb16`) to candidate (`b4f31bfe`) native Credential
      Manager succession (9.44s), all twelve client stages (228.99s), fourteen
      general Codex selections and a file-tool loop (5.99s). Observed clients are
      Codex 0.159.3, Claude 2.1.286 and official Hermes `f97608f` / 0.21.5.
      Exact safe producer output is in `4dd8504a/proposal-publication/github-windows-*`;
      consumer hashes and final archive/provenance are not independently replayed.
      Earlier Windows ARM64 results retain their scope; final bytes belong to 9.3,
      stable-channel security to 6.5, and Runner containment to 7.4.
- [ ] 5.4 Run equivalent macOS journeys with native Keychain authorization and
      environment backend separately in isolated acceptance; final operator-item
      authorization remains required before post-archive installed cutover.
      No password/biometric retry loop, service restart, or hidden native-store
      policy change is permitted.
      Current-source disposable Intel macOS run 37122724491 / job 111201877396
      passes published 0.3.1 (`ffc89cb9`) to candidate (`7b2cb1fa`) synthetic
      Keychain succession: original subtest RUN/PASS 22.90s, parent 35.78s.
      Retained readers, rotation, rollback, re-upgrade and cleanup are verified;
      `4dd8504a/proposal-publication/github-keychain-peer-*` retains safe markers.
      The selected `25006e50` environment-backend lifecycle passes separately.
      Remaining: final selected native-store bytes and stable Developer ID reader
      identity. Neither result proves operator-item access or Homebrew replacement.
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
      Authored Go/npm/Mise/OpenSpec pins, lock resolution and registry signatures
      are qualified at the current stable-compatible versions. The newly reported
      High advisory reopens final security admission in 6.5; latestness is not a
      vulnerability fix and neither a transitive compatibility violation nor a
      vendor patch is admitted.
- [x] 6.2 Make Go format, lint, static analysis, test, race and coverage rules
      cover product, tools and tests with zero unowned warnings; remove duplicate
      custom rules when a mature native tool proves the same property.
      Both peers' native Go gates pass at signed `4dd8504a`, including actual
      Intel macOS. Current producer coverage is 13,752/14,454 (95.14%) on GitHub
      macOS and 13,634/14,341 (95.07%) on GitLab Linux, with zero Go issues and
      required canonical-package observation. Safe summaries are in
      `4dd8504a/proposal-publication/`; raw hosted profiles were not exported.
      Native diagnostic-tail RED/GREEN and exact cleanup remain conserved. The
      cache invariant preserves its original two-minute budget and independent
      six-target archive test. Final security belongs to 6.5, not this Go scope.
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
      Blocking: `braces` 3.0.3 / `GHSA-vfj7-8cjw-p6xm` (High 8.7), reachable
      through supported OpenSpec custom-schema output expansion and Mermaid
      dependencies. Official metadata lists no patched stable version. No ignore,
      severity downgrade, vendor patch, fork substitution or schema restriction
      is admitted. Both current review quality jobs fail this same root advisory.
      Removing the Markdown CLI intermediary deletes seventeen packages while
      preserving native behavior; it does not repair this remaining chain.
      Existing SBOM/license/signature/provenance receipts bind their original
      candidates. The release owner must qualify the final exact locked matrix;
      its security refusal is not bypassed to manufacture signed assets.
- [ ] 6.6 Measure startup, setup, sync, credential read, native projection,
      build and CI costs against predecessor budgets on a quiet host; preserve raw
      samples and optimize only diagnosed owners.
      Existing Hyperfine reversed blocks retain forty samples per block,
      native memory calibration and warning admission. Prior `current-ebf-performance/`
      samples are inconclusive under contention and retain their raw outliers.
      Current quiet-host, native-store, construction and CI budget acceptance
      remains open; no unchanged noisy rerun or final-byte relabelling is valid.
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
      Existing proposal refs on both peers are signed `4dd8504a`. GitLab
      MR !178 pipeline 9400 and GitHub PR #162 run 37120920504 pass macOS, Linux,
      Windows and Secret Service; quality fails the High in 6.5. Actual Intel
      job 111196700148 succeeds in 806s without relaxing the cold-cache budget.
      All watchers are terminal. Historical `82f66fe9` Intel timeout stays
      failure evidence, not current failure; hardware/contention cause is unproved.
      Remaining: final exact-source successful review/accepted events and guarded
      integration. Manual native results never replace canonical required checks.
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
      Selected `25006e50` native macOS offline setup/catalogue/export/uninstall
      and sole-peer local Git controls pass with networking denied. Current-lock
      registry coverage binds all immutable tool inputs; it is not availability.
      Run 37122724491 proves one cold macOS Intel peer acquisition with cache
      disabled, concrete URL substitution and locked glab installation.
      Full platform/tool graphs, upstream-outage denial and GitLab Job Token
      downloads remain unproved; this manual scope does not close canonical CI.

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
- [x] 8.3 Review diagrams, tables, headings, lists, examples, CLI help and
      errors for readable layout and accurate links; enforce the chosen native
      formatter/linter rather than adding one-off checks.
      Current root-help misalignment is disproved: native and renamed commands
      align at display column 34. The regression covers widths and color modes
      without changing the renderer. Native text gates retain distinguishing
      blank-line/list controls and leave immutable archive bytes unchanged.
- [x] 8.3.1 Correct neutral Changelog headings, explicit peer history, and native repository locators without changing existing historical notes; verify strict release metadata and each actual peer destination. Keep the prepared release links distinct from unpublished tags.
      All 60 headings and 120 explicit peer links preserve historical bodies;
      referenced tags exist on their selected peer. Local-only v0.3.2 remains
      unlisted and prepared v0.3.3 does not claim publication. Historical object
      identity and installed prevention remain in 8.3.2.
- [ ] 8.3.2 Qualify the accepted installed ETHOS identity/reference contract, reject reachable wrong-repository destinations, and reconcile the audited historical peer tag identities through authorized native repair without a private map or copied checker.
- [x] 8.4 Compare mature gateway, config, client and release libraries with
      retained AIGW differentiators; record one source-backed adopt/reject decision
      per candidate and delete any replaced hand-written owner.
- [ ] 8.5 Inventory obsolete branches, generated outputs, records, stale tags,
      caches and worktrees by exact owner and consumer; retire only proved
      disposable items while retaining immutable evidence and running clients.
      Exact native retirement and duplicate-archive sweeps preserve running
      clients, selected bytes and immutable evidence. Original receipts prove
      181,380,500 and 117,200,868 bytes removed in separate local batches;
      Fleet's stopped Windows public-input batch removes 59,580,603 bytes.
      The obsolete GitLab proposal remains reachable but not accepted in dev;
      native `proposal_retirement_not_accepted` is respected. Remaining:
      accepted-source proposal withdrawal and final tag/output/lane retirement.
      Unknown caches and intentional dev/candidate/current worktrees are preserved.
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
      Selected macOS `25006e50` program `1426c66f` / archive `ec78a678` passes
      environment lifecycle and real-client scopes in 4.5. All 143 application
      build inputs/modes remain equal through signed `8b3b77c4` (`2d687aa5`);
      provenance, epoch and signatures are not rebound by input equality.
      Current-source native macOS/Windows succession is accepted in 5.3/5.4.
      Remaining: one final signed three-platform matrix, current Linux and final
      byte-bound clients/native stores, stable reader identity, full security,
      provenance and performance. Production signing/publication/installed
      Homebrew cutover remain separate [Migration Plan](design.md#migration-plan)
      acceptance; a source-green result or ad-hoc package does not satisfy them.
