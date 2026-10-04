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
      The exact signed macOS native-store and environment-reader succession
      now pass in 5.4/9.3. Other-platform native-store conjunctions remain open;
      an earlier ad-hoc reader or matching signer cannot prove item access.
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
      Current signed-package macOS client succession is recorded in 9.3. Earlier
      live-provider results retain their source-bound receipts below. The current
      Codex metadata audit identifies 25 Routes across 11 non-GPT Models without
      native metadata and three DMXAPI Routes requiring canonical alias projection;
      client acceptance remains Route-specific, including Sol 6.1 CDX.
      [catalogue evidence](../../../docs/research/provider-model-qualification.md#codex-native-chooser)
      owns the count and its limits. The explicit Sol 6.1 selection remains intact.
      Current Claude 2.1.288 passes public setup/use/check/verify, Read tools and
      same-session recall on three Sonnet 5.5 Routes (DMXAPI CC) and UCloud Opus
      5.5. Current Codex program/team/client bytes match retained AIHubMix and
      UCloud Sol 6.1 continuations; the missing DMXAPI CDX cell passes in 31.33s.
      Current installed Hermes `75e98367` also passes selected UCloud Sol 6.1
      inference, native file tools, same-session recall and public verification
      with the same candidate/team bytes; its eight carried commits remain
      explicit, not unmodified-upstream or final signed-byte qualification.
      Earlier client evidence and exact cleanup are bound in
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
      Current Windows official-client run 37189658374 at signed `f1eaa269`
      fails before inference: Hermes overrides caller Git environment policy and
      its fresh checkout then refuses the pinned commit. Exact supply cleanup
      succeeds; trust and lifecycle are not executed. The original failure and
      locked-installer causal review remain in the AIGW recovery owner. The
      caller-owned private Git config repair is accepted by the following run.
      Corrected run 37190612507 at signed `fa19f76d` passes official supply,
      authentic 0.3.1 Credential Manager succession, all three clients' twelve
      retained-state stages, team activation and five resource cases including
      the real 60-second deadline; exact supply cleanup passes. Independent
      receipt `aigw-fa19f76d-windows-corrected-native-acceptance` preserves the
      original log and earlier failure. Its loopback/source-built scope does
      not qualify final package custody, external Providers, Desktop UI or
      general Codex Routes whose native metadata warnings remain unresolved.
      The original Windows job now owns reviewed public-package acquisition and
      private glab/managed-Python preparation. Eight focused admission/tool-scope
      cases and the full projection suite pass; locked uv, signer inputs, format
      and size checks retain native results in `native-declarative-acceptance/`.
      This closes preparation only: exact final Windows package/client execution
      and retirement of superseded installer stages remain unproved.
      Hermes tools/history retain original bytes. Isolated `1426c66f` projects all
      eight Hermes wire catalogs and a second preview is unchanged; selections,
      comments and unrelated settings survive, but all eight reader paths change.
      Authentic 0.3.1 cannot project those wire IDs. Independent 2026-10-04
      catalog-replay and Claude-bundle receipts remain in the AIGW recovery owner.
      Claude Desktop 2.19675.0 has complete file/link identity and strict native
      signature evidence; dated Chat/Cowork/Code observations preserve launcher/
      profile continuity without qualifying final-mode use. Current signed
      reader and GUI qualification remain open. Final signed/store qualification
      stays in 9.3; operator/GUI/Homebrew cutover follows the [Migration Plan](design.md#migration-plan).
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
      not suppressed. The exact final Linux ARM64 `89974d7c` package now passes
      retained 0.3.1 published-predecessor acceptance in 1.30s, Claude 2.1.288
      and official Hermes 0.21.5 in 179.96s, and all five resource cases including
      the real 60-second deadline. Original raw results and unchanged-input,
      foreign-container and exact-retirement observations remain in
      `native-declarative-acceptance/current-linux-missing-client-corrected-20261004/`.
      The earlier overall failure preserves a caller-omitted archive; only its
      failed predecessor and never-run client scopes were repeated. Passed
      resources and Codex cells were reused. Remaining: exact native-host/store
      qualification. UID 1003/Runner 105 is not neutral; private DBus alone does
      not establish store isolation.
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
- [x] 5.4 Run equivalent macOS journeys with native Keychain authorization and
      environment backend separately in isolated acceptance; final operator-item
      authorization remains required before post-archive installed cutover.
      No password/biometric retry loop, service restart, or hidden native-store
      policy change is permitted.
      Exact ARM64 Developer ID candidate `03f03849` and authentic 0.3.1
      `ebfa8775` pass the original native Keychain retained-reader, rotation,
      rollback, re-upgrade and cleanup journey in 27.95s. The existing ordinary
      UID 504 SSH Security Session fixes the earlier root-session write refusal;
      no product fallback or ACL workaround was added. Fleet's
      `AIGW4b3f-Mac-authenticated-SSH-native-Keychain-final-custody-20261004T1035Z.json`
      preserves raw output and exact restoration; the retirement receipt proves
      owned stage removal. Independent review verifies candidate/tester custody.
      The separate signed environment-backend lifecycle passes in 9.3. Earlier
      Intel evidence remains in `4dd8504a/proposal-publication/`; none proves
      operator-item authorization, Desktop modes or Homebrew replacement.
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
      Normal signed `428f583f` checkpoints the latest stable-compatible authored
      Go/npm/Mise/OpenSpec pins, complete native locks and registry signatures.
      Native npm resolves the same graph;
      stable parents constrain three newer transitive suggestions, and Go's newer
      consumed suggestion is untagged development. Retain parents' pins: neither
      transitive compatibility violations nor vendor patches are admitted.
      Current upstream evidence remains in `13e6525c/current-upstream-review-20261004/`;
      latestness does not close security 6.5.
- [x] 6.2 Make Go format, lint, static analysis, test, race and coverage rules
      cover product, tools and tests with zero unowned warnings; remove duplicate
      custom rules when a mature native tool proves the same property.
      Signed `86cd24fe` passes both natural review matrices: GitHub run
      37184615068 and GitLab pipeline 9463. Six retained original atomic profiles
      agree on each platform's source ranges, statement counts and hit flags:
      macOS 14,022/14,737 (95.15%), Linux 13,904/14,624 (95.08%) and Windows
      14,060/14,796 (95.03%), all strictly above the unchanged 95% floor with every
      measurable canonical package executed. Native Windows retains its declared
      no-race mode; macOS/Linux and focused storage tests exercise race checks.
      Complete same-HEAD proof passes with the exact installed Publisher inputs.
      Storage/path refusal tests preserve unowned identity, content, mode and
      residue; two weaker tests are removed without changing product code or
      policy. Earlier coverage omissions, failed native attempts, static failure
      and caller/input mistakes remain evidence, not successful qualifications.
      Source/refusal evidence is in `5c90c796/storage-refusal-contract/`; exact
      proof, publication and cleanup in `86cd24fe/proposal-publication/`. The
      independent dual-peer matrix and original profiles remain with the existing
      AIGW recovery owner. Final release, security, clients, protected events and
      cold full-tool graphs retain their separate open tasks.
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
- [x] 6.5 Verify dependency hygiene, dead-code, secrets, SBOM, vulnerabilities,
      licenses, checksums, signatures and provenance from the exact locked
      candidate; delete unconsumed parallel scanners or reports.
      Current native Renovate observes CUE Codex 0.160.0/Claude 2.1.288 pins without
      relaxing release-age/automated proposal policy. Syft 1.54.0 six-platform
      provenance locks and real SBOM tests pass; source receipt is in
      `13e6525c/current-upstream-review-20261004/`. Final native client scope is in
      4.5/5.2; current signed-package custody is recorded in 9.3.
      Supported OpenSpec/Mermaid still consume `braces` 3.0.3,
      [GHSA-vfj7-8cjw-p6xm](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm)
      (CVSS 4.0: 8.7); native deep-schema/output probes reproduce the failure.
      No official stable fix is observed; [PR #72](https://github.com/micromatch/braces/pull/72)
      remains unmerged. The operator explicitly approved only reviewed
      development-tool inputs on 2026-10-04, not production or arbitrary
      untrusted input. The native OSV policy expires on 2026-10-18 and retires
      sooner on changed package/group, an obsolete finding or a reported stable
      fix. No downgrade, fork, broad package ignore or other-gate waiver is used.
      Quality and construction share the original dependency-evidence owner:
      two native scans retain complete raw findings and separate disposition,
      exact package/license inventory, native diagnostics and exit evidence.
      Raw scope is checked against the approved npm/braces/3.0.3/dev lock before
      disposition; malformed/missing/incomplete output and native warnings refuse
      qualification. Both Forge projections retain evidence after failure.
      The real current-lock source journey passes in 11.90s with 309 package
      identities and the raw finding retained; native controls and original
      failed journeys remain in `4b8f3452/native-dependency-closure/`.
      The earlier `b2dbeeb7` 7.06s refusal stays valid for its original policy.
      The exact `4b3fc946` signed candidate now passes dependency hygiene,
      dead-code and secret checks, six-program SBOM binding, the complete
      309-identity license inventory, checksums, explicit artifact-signature
      trust and source provenance. The original raw/disposition scans remain
      inputs to these consumers, not disposable parallel reports. Independent
      scope acceptance is retained in the AIGW recovery owner. This bounded
      development-input disposition is not a vulnerability repair, production
      waiver or VEX claim. Native product journeys remain in 3.5, 4.5,
      5.2-5.4 and 9.3; performance remains in 6.6.
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
      The `4b3fc946` ad-hoc matrix completes 32 blocks and 1,280 samples in
      69.684s; absolute budgets and memory calibration pass, but six native
      Hyperfine warnings keep acceptance inconclusive. Raw JSON, logs and
      summary remain in `4b3fc946/performance-env/`; exact scratch is removed.
      Simultaneous host load is observed, not proved failure causality. The
      Developer ID ARM64 measurement completes in 78.208s but fails with five
      warnings and credential block-2 p95 103.881ms above 100ms. All 1,280
      samples remain in Fleet's complete raw custody (`608b2dc8`). Short native
      CPU diagnosis does not identify a resolver hotspot or prove the cause of
      cross-block wall/CPU changes; no cache or threshold change is admitted.
      CPU profiling diagnoses eager full-executable identity reads in the default
      CLI constructor. Native invocation-local memoization removes that work from
      non-reader commands without a daemon, launcher or disk/global cache;
      explicit paths, failed identities and native projection/rollback remain
      governed by the same owner. Synthetic macOS `--version` Hyperfine reports
      16.1ms to 7.3ms mean (40 runs, five warmups each, no warnings); the original
      constructor benchmark moves from 6.96–7.22ms to 4.50–5.05µs, not an installed
      product claim. The complete native coverage attempt passes at 95.08%
      (14,009/14,734 statements); subsequent independent review reproduces and
      closes explicit native-auth-to-account-token selection with focused
      RED/GREEN. Exact raw profiles, original source-provenance refusal, fixture
      failure and review remain in `3f4b9c1c/startup-identity/`. Final signed
      artifacts, other platforms and native stores remain open.
- [x] 6.7 Assert warnings and malformed public errors fail at their origin;
      human and JSON output must expose precise state and safe action without
      private paths, Token fragments, internal modules or tracebacks.
      Selected `25006e50` Grok refusal, clean Sol verification, nonempty
      checkpoint preservation and Codex/Claude retained-state regressions pass
      (`27e761ad`, `faa46eab`). Native capture rejects explicit diagnostics while
      preserving normal progress; original failed controls remain retained.
      Original actionlint 1.7.12 writes ShellCheck stdin before starting its
      child; native oversized-input tests time out and the captured stack proves
      this owner-level failure. The same workflow gate now retains actionlint
      schema/expression checks and runs mandatory native ShellCheck through the
      existing bounded process owner. CUE declares shell defaults; native YAML
      enumerates every generated run rather than guessing runners. All 31 run
      identities are recorded, 16 Bash/sh scripts are checked and 15 PowerShell
      scripts retain their native classification. The real gate completes in
      0.53s; large-input, actual shell-defect, expression and inheritance controls
      pass. No wrapper, fork, minification or disabled shell-validation claim.
      Original timeouts and native source evidence stay in
      `108e3c75/native-workflow-input/` and `4b8f3452/native-dependency-closure/`.
      Coverage tests now execute the original workflow and dependency owners
      directly, including native tools and exact evidence/resource failures;
      a subprocess success no longer substitutes for measured implementation.
      The full macOS source gate passes at 95.04% (13,956/14,684 statements),
      strictly above the unchanged 95% floor. Original profiles and failures
      remain in `3f4b9c1c/coverage-owner/`. Source and native commands now retain
      the original Go profile under one verification directory, projected to both
      peers' always-upload artifacts; path-mismatch RED/GREEN covers macOS and
      Linux. This is not final candidate or other-platform acceptance.

## 7. CI and Dual-Peer Admission

- [x] 7.1 Reconcile the complete CUE CI graph with generated GitHub and GitLab
      projections; prove no hand-edited workflow drift or missing source, native,
      release or publication owner.
- [x] 7.2 Prove both peers' exact-SHA event-to-check contract for developer
      proposal create/update/review, maintainer fast-forward, accepted `dev`/`main`,
      and signed-tag pushes. Run the exact candidate's proposal checks on both
      peers; CUE and projection regressions must bind each other event to its
      intended commit SHA and required-job set. Actual `main`/tag results against
      the archived SHA are post-archive release acceptance under Migration Plan;
      do not infer them from projection tests or a manual run.
      Signed `4b3fc946` passes the natural review events on both peers: GitHub
      PR #162/run 37192570764 and GitLab MR !178/pipeline 9488 each pass all
      five required jobs without retry. Six original atomic profiles agree
      per platform, execute all 59 measurable packages and exceed 95%:
      macOS 95.148925%, Linux 95.077260%, Windows 95.026355%. Independent
      custody remains in the AIGW recovery owner. The existing CUE routing,
      exact checkout, event/base, protected parity and publication-identity
      regressions pass; raw results are in the source-bound
      `native-declarative-acceptance/current-ci-*.log`. These prove pre-archive
      admission only. Actual guarded integration and source-ref deletion precede
      archive in the Migration Plan; archived main/tag events follow it. Earlier
      failed attempts retain their original receipts; none is relabelled.
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
      This candidate prerequisite requires effective enforcement and native
      containment, not its own later integration effects. The Migration Plan
      requires actual guarded `dev` integration, `dev` checks and exact proposal
      source-ref deletion before archive, then archived `main`/tag events.
      A policy readback proves configuration, not any of those effects.
      Required check names, strict/admin enforcement, GitLab merge policy and
      unprotected proposals are observed; configuration is not enforcement proof.
      CUE separates protected/unprotected Linux registrations. The October 4
      native identity receipt still observes an elevated original Windows Shell
      token; a temporary restricted derivative does not qualify actual job
      containment. macOS account/permission metadata likewise lacks a current
      job's credential-carrier denial witness. Remaining candidate acceptance:
      effective required-status admission and untrusted Shell credential
      containment. Actual protected events, merge and source-ref deletion remain
      mandatory at their respective Migration Plan boundaries.
- [ ] 7.5 Test one-peer-only and offline-local operation; an unavailable GitLab
      or GitHub must not turn the other selected peer or the local product into a
      hidden dependency, and parity claims remain peer-specific.
      Current networking-denied native install/setup/status/doctor/catalog/preview/
      export/uninstall passes with 57 Routes/3 Accounts preserved and absent
      credentials/clients deferred; independent exact readback/cleanup is in
      `13e6525c/offline-current-native-product-20261004/`. Earlier sole-peer local
      Git controls keep their candidate. Lock coverage is identity, not availability.
      Run 37122724491 proves cold Intel macOS URL substitution/locked glab.
      Current exact-source GitLab peer mode installs 18/19/18 locked tools and
      passes Linux/macOS/Windows; the actual UID 504 executables use the owned
      job mirror. Fresh GitLab metadata matches all 94 locked archive references
      (90 files); six projection/cleanup race tests pass in
      `428f583f/peer-tool-independence-20261004T013302Z/`. The 94 API fallback
      references have no mirrored files; missing copies may fail locally by
      contract. Native macOS Mise observes both locked download/API paths under
      a synthetic peer returning 404, with exact cleanup; a temporary omitted-API
      mapping is rejected. The same native test and cleanup pass on Linux ARM64
      using locked Mise 2026.10.1, network denial, an unprivileged user and a
      read-only source mount; container removal is verified in
      `07bddf38/linux-peer-regression/`. Static gates pass with unchanged thresholds. Metadata
      and synthetic peer execution do not prove downloaded bytes,
      provenance, full cold platform graphs or upstream-outage containment.

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
      The full source gate passes its current native security disposition and
      coverage floor; final locked-candidate security/provenance remains in 6.5.
- [ ] 9.3 Qualify one signed untagged macOS/Linux/Windows candidate with
      exact-byte install, predecessor upgrade/rollback, credentials and real-client
      journeys; retain the source and artifact hashes and disclose missing platform
      evidence.
      Native/client evidence is owned by 3.5, 4.5 and 5.2-5.4; exact security and
      provenance by 6.5, performance by 6.6, peer enforcement by 7.2-7.5. Each
      retains its candidate identity. Current `4b3fc946/developer-id-dist/` has
      six archives, complete SBOM/license/vulnerability/provenance evidence and
      an independently verified checksum signature. Both macOS executables
      pass the exact Developer ID requirement, Team, runtime and timestamp
      checks. Apple submission `a3b623ee-8acb-4574-8a8f-0f255a4f21e6` accepts
      their exact ZIP; the native final-distribution verifier passes with no
      issues. Original results remain in `4b3fc946/notarization/`.
      The signed macOS ARM64 package passes the existing lifecycle/resource,
      published-predecessor and real-client suites in 110.473s, 8.343s and
      198.546s. Codex 0.160.0, Claude Code 2.1.289 and local Hermes 0.21.5
      (+9 carried commits) pass all twelve retained-state stages. The resource
      cases include the real 60-second deadline. Exact scratch and owned
      processes are removed; raw results remain in the existing
      `native-declarative-acceptance/current-developer-id-native-macos.log`.
      Environment credentials and a loopback Provider do not qualify native
      stores or external inference. The separate exact synthetic Keychain
      succession now passes in 5.4. The same Linux ARM64 package passes authentic
      0.3.1 retained-state Codex/tool-loop container acceptance in 69.31s;
      `current-linux-container-native.log` retains the original result. Exact
      container/scratch removal and unchanged foreign inputs are verified. The
      same exact package now also passes official Claude/Hermes retained-state
      container journeys, published predecessor and five resource cases in 5.2.
      Claude Desktop's deferred-installation
      case is skipped on this already-installed host; general Codex Route
      metadata warnings remain explicit. Remaining: Linux native-host/store
      and Windows package acceptance, official client and Desktop modes,
      performance, peer containment and cold supply. Production publication
      and installed Homebrew cutover follow the [Migration Plan](design.md#migration-plan).
      Equal product inputs do not rebind signatures, source epoch or provenance.
