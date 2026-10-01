# Tasks

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
- [x] 2.7 Admit a selected live Account and client through endpoint and
      inference checks, while classifying absent native authorization without
      prompts or backend fallback; keep supplier-specific failures scoped to that
      Account. A focused `check --for` must not observe or charge another
      enabled client's Account.
      On September 30, isolated public setup from the shipped team manifest
      selected DMXAPI GPT-6 Sol for Codex. Current product source at `adb83383`
      returned `check --for codex --json` with one enabled client and healthy
      inference through the direct endpoint; subsequent commits changed only
      research prose. Current-HEAD focused tests verify single-Account Token and
      endpoint scope, quota classification, denied credential metadata without
      fallback, and locked-Keychain no-prompt behavior. Signed artifact and
      real-client acceptance remain in 4.5 and 9.3.

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
      At signed source `cdfc0bf6`, the isolated macOS environment-backend
      journey stages actual Homebrew 0.3.1 bytes and passes preprojection,
      simulated link gap, upgrade, rollback, re-upgrade and uninstall. New
      readers survive the gap; cached 0.3.1 callers retain the disclosed risk.
      The signed published 0.3.1 Linux ARM64 archive passes the corresponding
      environment-backend journey in an isolated container. GitHub run
      36563790078 at `49ba0c5ec5503b4365ff1f0c01555c05c97ca03e` passed
      the published `v0.3.1` predecessor Keychain journey on a disposable macOS
      runner, including rollback rotation and explicit restaging. Operator-item
      authorization, Linux host credential service and Windows real-client
      proof remain open.
      The signed `2c7fbe89` Linux ARM64 candidate also passes authentic 0.3.1
      succession with an isolated real DBus/GNOME Secret Service as UID 1000.
      Automatic backend selection is keyring; old and new commands remain
      callable through upgrade, rollback and re-upgrade, and uninstall retains
      the stored Token. Linux host and Windows evidence remain separate.
      October 1 private-Keychain acceptance proves that a same-byte copied test
      executable reads its creator-authorized synthetic item and observes parent
      rotation/deletion. A missing selected executable fails; ignoring that
      selection produces the distinguishing RED. This is not production
      signing succession or authorization of retained operator items.
      The unchanged `ebf9adff` Linux candidate also passes the signed
      `00e58c70` system-store fixture with a real isolated Secret Service and
      published 0.3.1 predecessor; see the native-store scope in 5.2.
      Native Windows Credential Manager also passes the unchanged candidate's
      retained-reader fixture at signed `00e58c70`; see 5.3. These isolated
      synthetic items do not authorize the final operator reader.
      Current `3f6723c7` macOS run 36851755292 passes authentic published
      0.3.1 Keychain succession, rollback rotation and explicit restaging;
      Windows native-store/client closure is recorded in 5.3. The three
      isolated platform results remain separate from the final exact operator
      reader and the actual Homebrew link transition, which are still open.
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
      Signed `80dbca19` and its exact 0.3.3 candidate contain 26
      Models and 58 Routes. The independent October 1 public setup/use/check
      matrix passes every exact Account/wire/protocol pair once: DMXAPI 18/18,
      UCloud 15/15 and AIHubMix 25/25; 37 Responses, 11 Chat and 10 Anthropic
      routes. No-account setup remains Deferred with `ok=false`. Source,
      candidate and twelve protected operator identities remain unchanged;
      owned processes and 113,841,464-byte scratch are absent. Receipt
      `independent-current-provider-80db-20261001-01a0ccfc/delivery.json`,
      SHA-256 `9ffa097b805addc16e1fb9bc1f8c63f18448597d94eb95d2cbbf672083b6ab63`,
      is retained under the existing AIGW recovery owner. This proves dated
      inference, not sustained availability, authentic client continuation,
      global ranking or installed cutover. Native evidence for those scopes
      stays in 4.4–4.5 and 9.3.
      [Catalogue qualification](../../../docs/research/provider-model-qualification.md#provider-catalogue-and-route-evidence)
      retains withdrawn unavailable Routes, the plain DMXAPI Sol 6.1
      continuation limit and MiniMax's native Codex limit. Qualified CDX/CC
      alternatives share existing logical Models. Guarded import/add
      regressions preserve sparse Accounts, explicit selections, external
      credential commands, shared Models and exact upstream wire IDs; invalid
      or dangling recommendations fail before mutation. Historical artifact
      evidence is retained with its original identity, not copied as current.
      Later exact `444bbd41` candidate and official Hermes `f97608f` native
      file-tool/same-session acceptance qualify Ultra and `cohere-command-a`;
      the dated Cohere wire returns no tool calls. Replace only the shipped
      Super/datetime choices, preserving explicit installed selections and
      protocol ownership. Configuration regressions and all seven shipped-
      team setup journeys pass. The updated catalogue still has 26 Models
      and 58 Routes; current artifact and platform qualification remain open.
      Evidence: `independent-hermes-request-boundary-444-20261001-01a0ccfc`
      in the existing recovery owner; `native-model-catalogue-*` and
      `native-model-shipped-team-journey.log` in the existing verification owner.
- [x] 4.3 Prove that ordered existing recommendations select only usable
      unselected Routes, preserve explicit choices, and exclude manual-only
      Providers; no control-plane command claims request-time failover.
- [x] 4.4 Qualify Codex with selected non-OpenAI-family Responses-compatible
      models using its actual model chooser, authentication and tool loop; state
      native limitations instead of forging a model list.
- [ ] 4.5 Requalify Claude Code, Claude Desktop, Codex, and Hermes independently
      for native protocol, model selection, credential and rollback behavior; do not
      infer Desktop from CLI or endpoint reachability from real-client success.
      Claude-in-Codex remains unadmitted: shipped Claude Routes expose Anthropic,
      while Codex requires Responses. Qualify an exact external Responses path
      and real Codex tool/replay behavior before claiming that support.
      The exact `ebf9adff` candidate passes macOS authentic 0.3.1 succession
      with Codex 0.159.3 and Claude Code 2.1.286, including retained inference,
      replacement, rollback, re-upgrade, native settings/MCP and external-reader
      preservation. Claude 2.1.286 is latest-channel compatibility; stable
      remains 2.1.285. Fourteen general Responses Routes and the native Codex
      shell tool loop pass. The independent new-native `54b815ae` SDK receipt
      retains the prior failed sandbox/SDK attempts and proves exact input,
      candidate, source, host and scratch conservation. Official Hermes
      `f97608f` (0.21.5) already passes its unchanged four-stage journey.
      Separate September 30 direct DMXAPI and UCloud GPT-6.1 Sol Codex/Hermes
      sessions pass. Hermes verification now preserves configured per-model
      reasoning and native ownership. The October 1 exact-candidate public
      setup/use/verify journey with official Hermes `f97608f` preserves the
      AIHubMix Mistral override at none and high; each main streaming request
      carries the selected effort, with configuration and scratch conserved.
      Its independent receipt is `independent-hermes-mistral-ebf9adff-corrected`;
      the loopback probe does not qualify actual AIHubMix inference.
      The October 1 independent `ebf9adff` artifact review also completes live
      GPT-6.1 Sol inference and real Codex/official Hermes sessions on all three
      Accounts, plus stable Claude 2.1.285 verification of ten shipped
      Anthropic Routes. Protected files and exact scratch are conserved.
      AIHubMix Mistral accepts none; high fails with an unproved cause and no
      retry. These receipts qualify their exact inputs, not every model or
      later artifact. See `independent-ebf-live-provider-20261001-01a0ccfc`,
      `independent-ebf-claude-stable-20261001-01a0ccfc` and
      `independent-ebf-hermes-models-20261001-01a0ccfc` in the existing recovery
      handoff `20260930-sol61.VVgo1vF5`.
      Independent `9cf23cb8` AIHubMix client acceptance completes Sol 6.1 with
      Codex/official Hermes, Muse Spark with Codex, and Opus/Sonnet 5.5 with
      stable Claude. MiniMax M3 completes in official Hermes, but real Codex
      returns reasoning tags in its final text and fails the exact marker
      contract. Later official Codex 0.159.3 qualifies the exact AIHubMix
      `cc-minimax-m3` channel and UCloud `MiniMax-M3` under native effort none,
      with shell execution and same-thread context replay. Current `7a5c1da6`
      candidate public Route addition and projected-client verification also
      pass; the plain AIHubMix Route's client gap remains disclosed. See
      `independent-minimax-codex-native-20261001-01a0ccfc` and
      `independent-current-candidate-7a5c1da6-20261001-01a0ccfc`; no new
      logical Model, response stripping or marker relaxation is introduced.
      Linux real-client lifecycle and selection evidence is recorded in 5.2;
      final Windows bytes, native-host stores,
      Desktop GUI, installed-host cutover and final signed bytes remain open.
      The later independent `9cf23cb8` DMXAPI batch passes all seventeen Routes
      in direct inference, but real Codex 0.159.2 returns unrelated final text
      for Sol 6.1. Official SDK streaming/nonstreaming success is not client
      acceptance. UCloud's fifteen Routes and selected native clients pass.
      See `independent-9cf-dmxapi-ucloud-20261001-01a0ccfc` in the existing
      recovery handoff; the complete-request DMXAPI cause remains open.
      The independent `independent-dmxapi-prompt-diagnosis-20261001-01a0ccfc`
      delivery isolates native input item IDs: the same full nine-item tool
      continuation fails with IDs and passes after only the user-message ID
      is omitted; removing only a tool-call or tool-output ID does not fix it.
      UCloud accepts the original request. This is a measured compatibility
      trigger, not a proved provider-internal cause or native Codex repair;
      prompt duplication and AIGW-owned transport rewriting remain excluded.
      The separate nested-Seatbelt fixture defect is corrected. Raw requests,
      responses and reproducer remain; twelve protected inputs are unchanged,
      all owned children are terminal, and 232,554,214 scratch bytes are retired.
      Independent `independent-dmxapi-sol61-cdx-20261001-01a0ccfc` qualifies
      the exact `9cf23cb8` program and added CDX Route with official Codex
      0.159.3 public verification, shell execution and same-thread tool replay.
      `independent-dmxapi-cdx-hermes-20261001-01a0ccfc` separately qualifies
      unmodified official Hermes `f97608f`. Exact conservation and one preserved,
      unowned concurrent host edit are bounded in the
      [qualification owner](../../../docs/research/provider-model-qualification.md#provider-catalogue-and-route-evidence).
      Later-artifact, Desktop GUI and every-platform acceptance remain open.
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
      release bytes and one retained predecessor state. Prior signed container
      evidence covers cold bootstrap (`d7cc0c75`) and published 0.3.1 succession
      with environment, automatic secure-file fallback and real GNOME Secret
      Service (`2c7fbe89`, `d65dd140`); containers and owned scratch are removed.
      October 1 native Linux ARM64 UID 1006 executes the precompiled `90e13374`
      verifier against exact `ebf9adff` candidate and signed 0.3.1 predecessor.
      Selected artifact install/update/rollback/uninstall passes in 1.424 seconds;
      all five verification resource cases pass in 61.081 seconds, including the
      actual 60-second deadline. All 1,228 source and program hashes are conserved;
      guest child, processes and temporary transfer are absent, and twelve Runner
      pause states plus VM isolation are restored. This qualifies synthetic-client
      environment-backend native-host journeys, not actual clients, native Secret
      Service, cold-peer transport or final publication; those gaps remain open.
      Independent Linux ARM64 Docker acceptance uses signed `ea8d5d8f` verifier,
      unchanged `ebf9adff` bytes, Codex 0.159.3, stable Claude 2.1.285 and
      official Hermes 0.21.5. All three retained 0.3.1 client lifecycles and
      fourteen general Codex selections pass. Only the failed tool loop is
      repeated after native sandbox namespace preflight and passes in 6.05
      seconds; no client sandbox policy, host kernel or extra capabilities change.
      All 16,309 input hashes match; exact containers and scratch are absent.
      Combined lifecycle and focused-successor evidence is not a single green
      run, native-host Secret Service, live supplier or final-distribution proof.
      Independent fixed-candidate receipt `f6b0d496` adds the actual isolated
      Linux Secret Service journey as registered UID 1000 with network removed
      before execution. Signed `00e58c70` verifier, candidate and published
      0.3.1 inputs remain unchanged; retained-reader upgrade/rollback/re-upgrade,
      Account/diagnostic rotation, rename, uninstall/reinstall and deletion pass.
      All 1,228 source hashes match, and container, scratch and temporary recipe
      are absent. An official `secret-tool`-only counterexample reproduces the
      GNOME Keyring 42.1 duplicate-registration diagnostic without AIGW. These
      daemon warnings remain raw evidence, not a product fault or warning-free
      qualification. Native-host, real-provider and final-distribution claims
      remain open.
      `independent-9cf-linux-codex-claude-20261001` in recovery handoff
      `20260930-sol61.VVgo1vF5` completes the exact `9cf23cb8` Linux ARM64
      candidate's retained 0.3.1 Codex 0.159.3/stable Claude 2.1.285 lifecycles,
      fourteen Responses selections and shell tool loop in one 152.769-second
      run. All 1,229 source hashes and program identities are conserved;
      exact container, scratch and owned processes are absent. This is loopback
      container evidence, not native-host Secret Service, Hermes or final bytes.
- [x] 5.3 Run equivalent Windows native journeys, including ACL, executable
      replacement, path quoting, noninteractive environment Token and installed
      Codex/Claude consumers; report each unproved client mode explicitly.
      Exact `ebf9adff` Windows ARM64 candidate passes core, shipped-team,
      rollback and authentic 0.3.1 succession under the actual Runner 103
      service account. Retained-command inspection preserves the OS short-name
      namespace; parser and native identity regressions pass. The October 1
      native acceptance closeout records exact archive/program identity, no
      owned processes or child directory, and restored service, pauses and
      VM isolation. Native Go cache cleanup followed by exact read-only Git
      pack teardown closes the earlier cleanup failure without a product
      rerun. Real clients, performance and arbitrary-code
      review containment remain unqualified.
      October 1 signed `00e58c70` verifier also passes the unchanged candidate's
      portable lifecycle in 5.58 seconds and all five resource cases in 62.91
      seconds. Receipt `3cdaeb1c` binds 1,244 inputs, including 1,228 signed
      source files; exact child/process cleanup preserves the serving identity,
      six Runner pause states and VM isolation. These are environment-backend
      journeys, not Credential Manager or actual-client acceptance.
      The separate native Credential Manager journey now passes in 7.64 seconds
      at signed `00e58c70` with exact candidate and published 0.3.1 bytes.
      Receipt `3ea7175b` binds the actual service identity, native backend,
      retained-reader upgrade/rollback/re-upgrade, rotation/rename/diagnostics
      and uninstall/reinstall. Both test teardown and independent outer cleanup
      prove the four exact owned synthetic slots absent; child, processes and
      listeners are reclaimed, with source hashes, serving identity, original
      pauses and VM isolation conserved. No operator Token is touched or logged.
      This success path does not prove native-store fault injection, real
      clients, Runner containment, performance or final distribution.
      Current signed `3f6723c7` closes the pre-archive Windows journey through
      GitHub native-client run 36849808698, job 110328519476. The exact Windows
      AMD64 candidate (program `ec7d9429`, archive `5e82274a`) passes core,
      shipped-team, native Credential Manager, all five resource outcomes and
      authentic published 0.3.1 succession. Actual Codex 0.159.3, Claude Code
      2.1.286 and pinned Hermes `f97608f` complete baseline, candidate, rollback
      and re-upgrade; thirteen current general Codex Routes and the shell loop
      pass in one 224.35-second client run, without failed/skipped tests or
      warnings. Product teardown asserts exact owned installation removal;
      disposable hosted cleanup completes. Both review peers also pass their
      required Windows jobs. Source-bound logs and final run/job readbacks are
      `3f6723c7-*` under `build/verification/supply-chain-20260930/`.
      Claude Desktop GUI remains in 4.5; performance, persistent Runner
      containment and final distributed-byte acceptance remain in 6.6, 7.4
      and 9.3. This closure does not claim those independent outcomes.
- [x] 5.4 Run equivalent macOS journeys with native Keychain authorization and
      environment backend separately in isolated acceptance; final operator-item
      authorization remains required before post-archive installed cutover.
      No password/biometric retry loop, service restart, or hidden native-store
      policy change is permitted. At signed
      `9f23bd7e`, private-Keychain tests prove readable labels on newly created
      Account and diagnostic items without touching retained items; the macOS
      source/native suite passes using isolated environment credentials. The
      installed-product native-store journey and Claude Desktop GUI remain
      unproved; this host skipped Desktop deferred installation because it is
      already installed.
      `d65dd140` macOS artifact lifecycle, rollback admission and shipped-team
      journey pass with the retained signed predecessor. Eighteen protected host
      identities remain unchanged; owned descendants and scratch are absent.
      Native-store, deferred Desktop installation and performance claims remain
      separate from this environment-backend acceptance.
      The private copied-test-reader evidence in 3.5 does not close final
      production-reader authorization or Desktop GUI acceptance.
      Disposable GitHub macOS run 36851755292 at `3f6723c7`, job 110334863497,
      now passes core, shipped-team, all resource failure outcomes and authentic
      published 0.3.1 Keychain succession without warnings or skipped tests.
      Its candidate program is `c38d9da4`; it does not authorize retained
      operator items. Current `9544b98d` local matrix separately passes
      environment-backend lifecycle and actual Codex 0.159.3/stable Claude
      2.1.285/installed Hermes succession in 235.212 seconds. One deferred
      Desktop test explicitly skips because Desktop is already installed;
      official Hermes and Desktop GUI remain independent in 4.5. Raw logs and
      exact caller inputs are `3f6723c7-macos-native-keychain*` and
      `native-current-9544b98d-macos-current-trust*` under the existing
      `build/verification/supply-chain-20260930/` owner. Final operator-item
      authorization and distributed-byte cutover remain open.
      Current signed `02449bd2` revalidation finds all 136 product inputs and
      nine native fixture/lock inputs byte-identical to the qualified macOS
      source, with no omitted product file. Fresh hosted readback confirms
      job 110334863497 completed successfully; published Keychain succession
      and the separate environment-backend journey establish this isolated
      pre-archive scope. Current shipped-team and process-observer regressions
      also pass. Evidence: `macos-isolated-acceptance-input-conservation-02449bd2.json`.
      Actual operator authorization and Homebrew cutover remain in the Migration Plan;
      Desktop GUI and official Hermes acceptance remain in 4.5.
- [x] 5.5 Verify missing Codex/Claude at setup and later installation on each
      platform; deferred sync must touch only installed admitted clients and
      preserve user-owned files. GitHub run 36560334613 at
      `49ba0c5ec5503b4365ff1f0c01555c05c97ca03e` passed
      the deferred-installation subtests without skips for Claude, Codex and
      Hermes on macOS/Linux/Windows and Claude Desktop on macOS/Windows. Each
      journey installs its isolated client after setup, preserves other Client
      Bindings and projections, and retains seeded user configuration through
      sync and uninstall. Published-byte and real-client claims remain in
      5.2-5.4 and 4.5.
- [x] 5.6 Compare owned process, helper, temporary, journal, build and
      client-projection resources before/after success, failure, timeout and
      interruption; exact teardown preserves active installations and evidence.
      The existing tracked fixture runs success, client failure, parent exit,
      native interrupt and the actual 60-second deadline against unchanged
      `ebf9adff` bytes. October 1 macOS receipt `6ebedb4d`, native Linux UID
      1006 receipt `50c98b70` at verifier `90e13374`, and native Windows receipt
      `3cdaeb1c` at verifier `00e58c70` all pass. Their relevant fixture blobs
      equal current source; Windows executes all five cases in one green run.
      Each preserves configuration, projections, readers, program and unrelated
      processes; owned verification homes and descendants are reclaimed.
      Raw RED evidence for Windows console interruption and failed assertion
      cleanup remains retained alongside their native/focused successors.
      Source/race regressions also cover cancellation, reader loss and cleanup's
      independent bounded context; full source coverage remains above 95%.
      This closes the isolated resource contract, not native-store, real-client,
      hosted CI, operator cutover or final-distribution acceptance, which remain
      at 4.5, 5.2-5.4, 7.x and 9.3.
      The current `d75eb6e9` artifact run exposed partial fixture publication:
      `parent.json` could exist before its JSON bytes were complete. The
      deterministic reader-inode regression fails the old producer and passes
      reuse of the existing atomic writer. The exact candidate then passes all
      five resource outcomes, including native interrupt and the product's
      sixty-second deadline, in 71.526 seconds. Race and observer siblings pass
      without new framework or product changes; owned scratch and descendants
      are absent. Evidence: `resource-atomic-publication-*` under the existing
      `build/verification/supply-chain-20260930/` owner. Corrected-fixture
      hosted platform acceptance remains separate in 7.3.

## 6. Quality, Supply Chain, and Performance

- [x] 6.1 Audit every direct Go, npm, OpenSpec, Mise and release-tool version
      against the latest stable compatible upstream; update authored pins and locks
      once, then prove clean-context reproducibility and license/security
      admissibility. The October 1 official audit and native producers bind
      fourteen direct Go dependencies, nineteen Mise tools, OpenSpec and text
      tooling to stable-compatible versions. npm 12.2.0, Mise 2026.9.18,
      GitHub CLI 2.102.0, glab 1.120.0 and Renovate 44.125.1 are admitted;
      locked npm resolution preserves parent constraints and the three-day
      transitive admission window instead of incompatible overrides.
      Bootstrap, registry verification (283 signatures and 51 attestations),
      both native lock resolutions and all 111 payload hashes pass. All 43 Go
      and 283 npm packages have established licenses and zero OSV findings.
      Both peers pass the five-job source/native matrix at signed `ea8d5d8f`.
      A fresh mutable workspace at `b4a825b8` reconstructs all 1,228 signed
      source inputs and npm dependencies through the existing bootstrap in
      2.36 seconds with network denied, private HOME/state/node_modules, and
      supported content-addressed caches. No authored hash changes or warnings;
      exact scratch is removed. This is source-supply reproducibility, not an
      empty-cache claim. Complete cold-peer execution and final product bytes
      remain open at 7.5 and 9.3; credential authorization is not a tool version.
      The later official OpenSpec 1.14.0 release is explicitly admitted for this
      user-requested stable maintenance; authored pin, native npm lock and local
      installation agree. Only that direct package bypasses the three-day age
      window; automatic policy and transitive constraints remain unchanged.
      Registry verification retains 283 signatures and 51 attestations, and the
      official eleven-item validation plus affected tool/artifact tests pass.
      The frozen full source gate passes at 95.09% with 1,228 hashes conserved;
      offline npm cache reconstruction preserves authored locks and installs the
      same 1.14.0 tool. Its owned compile-cache scratch is reclaimed.
      This repository tool upgrade does not update the separately bound ETHOS
      runtime or re-sign the fixed candidate.
      Public runtime repair later binds accepted ETHOS source `3a49b933`, wheel
      `6f17c97a` and immutable generation `00b138cd` across all three linked
      roots. Profile, OpenSpec config, repository state, source and installed
      product identities remain unchanged. The five toolchain tests that failed
      under the old runtime now pass in its corrected native environment; the
      old runtime's hidden `mise`-missing failures remain raw evidence. This
      does not resolve the separate documentation or tool-mirror gaps.
- [x] 6.2 Make Go format, lint, static analysis, test, race and coverage rules
      cover product, tools and tests with zero unowned warnings; remove duplicate
      custom rules when a mature native tool proves the same property.
- [x] 6.3 Measure ELOC, logical statements, nesting and complexity by semantic
      owner; set risk-justified blocking bounds and simplify real hotspots without
      mechanical file splitting or suppressions.
      October 1 Cyclop recalibration is closed: retain the blocking 25 limit
      across product, tools and tests. OS-selected macOS/Linux/Windows ARM64
      trials report zero findings at 25, 70/70/71 at 20, and 235/231/231 at 15.
      Counts alone do not prove defects. Review of atomic manifest admission,
      credential-directory cleanup, Windows ACL validation and lifecycle
      assertions found no demonstrated benefit for blanket reduction. No
      suppression, forwarding helper or assertion dependency was introduced;
      other limits and escaped-defect review remain active.
- [x] 6.4 Exercise Markdown, Mermaid rendering, internal/external links, TOML,
      YAML, JSON, CUE, shell and generated-text checks on tracked content; a
      malformed or unreachable authored carrier must fail the relevant gate.
      The Codex projection-copy regression now parses the actual TOML catalog
      path and verifies canonical destination ownership and unchanged source
      bytes, rather than comparing serialized Windows escape spelling.
      Release publication rejects a missing or non-directory parent before
      rename; focused tests preserve the candidate and operator-owned files.
      Windows native execution of these corrections remains required.
- [x] 6.5 Verify dependency hygiene, dead-code, secrets, SBOM, vulnerabilities,
      licenses, checksums, signatures and provenance from the exact locked
      candidate; delete unconsumed parallel scanners or reports. The signed
      `38246801` source builds six archives with a disposable test signer;
      matrix checksums and the detached signature verify, and provenance binds
      its commit, tree and four dependency locks. The SBOM has 221 packages;
      all 43 Go and 283 npm dependencies have known licenses and zero OSV
      findings. Native unused-code, secret, module and npm gates pass. Gitleaks,
      OSV and Syft have distinct single owners; historical candidate evidence
      is preserved, not duplicated into another scanner. Production signing and
      publication remain separate release obligations.
- [ ] 6.6 Measure startup, setup, sync, credential read, native projection,
      build and CI costs against predecessor budgets on a quiet host; preserve raw
      samples and optimize only diagnosed owners.
      Signed `bf35796c` removes repeated lifecycle suites from explicit
      performance dispatch without weakening ordinary acceptance. The existing
      measurement owner now includes first setup and converged sync, with
      distinguishing case, owned preparation and source-gate regressions.
      Cold-cache onboarding and construction/CI costs remain separate;
      current quiet-host measurements
      and budget qualification are still open.
      Actual 0.3.1 and candidate-byte smoke disproved setup reapply; corrected
      first-setup preparation and repeated setup/sync preserve exact helper
      and projection bytes. The invalidated source gate was stopped and its
      failure evidence retained, not accepted as performance qualification.
      The current exact candidate's environment-backend measurement executes
      both reversed forty-sample blocks: pooled p95 setup 43.509 ms, sync
      32.045 ms, projection 53.648 ms and credential 17.247 ms. Two native
      outlier warnings and host contention leave qualification inconclusive.
      Raw samples and summary remain in `current-ebf-performance/`; the
      existing acceptance owner now rejects native warnings without discarding
      evidence. Quiet-host, native-store, build and CI cost claims remain open.
- [x] 6.7 Assert warnings and malformed public errors fail at their origin;
      human and JSON output must expose precise state and safe action without
      private paths, Token fragments, internal modules or tracebacks.
      Candidate `ebf9adff` passes warning-free macOS native client replay and
      Linux package acceptance. Its public no-Token journey reports Deferred
      with `ok=false` and zero enabled clients; failed remote update preserves
      program/configuration, and partial uninstall distinguishes committed
      withdrawal from incomplete removal while retaining foreign content.
      Exact stdout/stderr and preservation results are retained under
      `build/verification/supply-chain-20260930/public-negative-ebf9adff/`.
      Unchanged source regressions cover restoration/finalization human and JSON
      output, private canary suppression and equal-version identity refusal.
      Platform, native-store and formal-distribution evidence remains in 5.x
      and 9.3; measured performance warnings remain in 6.6.

## 7. CI and Dual-Peer Admission

- [x] 7.1 Reconcile the complete CUE CI graph with generated GitHub and GitLab
      projections; prove no hand-edited workflow drift or missing source, native,
      release or publication owner.
- [ ] 7.2 Cover developer proposal create/update and review SHA, maintainer
      fast-forward, dev, main and tag events; each required check must execute on or
      attest the exact admitted object.
- [x] 7.3 Separate fast quality, locked bootstrap, macOS, Linux, Windows, native
      clients and release construction where independence saves time; share only
      immutable evidence and content-addressed caches. Cancel superseded review
      work only before a noninterruptible native Shell or release job starts.
      One release owner consumes peer-local candidate, predecessor, clients
      and trust inputs; construction/acceptance children inherit no Forge
      authorization. Current signed `df7d4585` passes all five required jobs
      independently on GitHub review 36887407579 and GitLab review 9174;
      GitHub full-quality run 36888622433 also passes the complete native
      quality graph on macOS, Linux and Windows. October 2 terminal readbacks
      bind each result to that exact SHA. Accepted-ref parity and release
      version are correctly skipped in review/manual contexts, not qualified
      by those runs. Focused CUE projection regressions pass, including cache
      isolation and noninterruptible persistent Shell jobs. Static quality and
      pure-Go Secret Service require no compiler; native Linux race alone
      installs gcc/libc headers. Original prerequisite, ESRCH and sandbox
      failures remain in `linux-prerequisite-*`, `linux-capability-*` and
      `windows-native-sandbox-*`. October 2 `df7` terminal readbacks remain
      in the existing verification owner. Event/merge admission,
      persistent-runner containment, cold-peer transport and final distributed
      client bytes remain in 7.2, 7.4, 7.5 and 9.3.
- [ ] 7.4 Prove required-status enforcement, unprotected proposal branches,
      guarded merge, source-ref deletion, signer trust and branch protection on each
      selected peer without interactive authentication or divergent commit
      identities. Prove untrusted review code cannot observe persistent Shell
      runner credentials or protected-job state; retain required native evidence.
      On September 30, GitHub dev/main require five GitHub Actions app-bound
      checks, including Linux Secret Service, with strict and admin enforcement;
      GitLab requires pipeline success, resolved discussions and source-branch
      deletion. The existing proposal ref remains unprotected on both peers.
      GitLab MR !178 pipeline 8887 passed macOS, Windows, Linux, quality, and
      Secret Service review jobs at `ddb998a5`; macOS/Windows project runners
      #105/#103 admit unprotected jobs. Their disposable execution, isolation
      from persistent credentials, and the final review SHA remain unproved.
      Keep proposals unprotected and verify those boundaries before admission.
      Linux runner selection now belongs to one CUE workflow rule set rather
      than an external project tag. Reviews and unprotected manual refs select
      the existing container runner; accepted refs and release tags select a
      separate protected container registration. All four Linux jobs consume
      that result. Focused projection tests and GitLab protected-ref simulation
      pass without warnings; protected registration, actual review/accepted
      jobs and retirement of the obsolete project variable remain open.
- [ ] 7.5 Test one-peer-only and offline-local operation; an unavailable GitLab
      or GitHub must not turn the other selected peer or the local product into a
      hidden dependency, and parity claims remain peer-specific.
      CUE binds peer selection, locked URLs and job-private credentials; local
      offline tests reject missing or altered mirror files. Native verification
      now qualifies all 21 provenance-bearing files in the 48-entry x64 lock
      scope, retaining the distinct Actions and vendor-release signer contracts.
      On October 2, the existing project-456 mirror gained 42 absent x64 assets
      (568,866,372 bytes). Each served byte string matches the current `mise.lock`
      SHA-256; all 54 prior registry-file hashes remain unchanged. Receipt
      `x64-peer-mirror-publication-verified-20261002.json` records the exact
      source and lock identities. Publication used the existing native glab
      identity; it does not prove CI Job Token identity or job-private auth.
      The earlier cold Windows 404 and corrected empty-cache pipeline 9172 remain
      distinct evidence. Deliberate sibling-peer outage, reciprocal download
      path, complete cold execution across native platforms and protected-runner
      execution remain open; review CI does not prove those conditions. Preserve
      the original 9171 input failure and `cold-tool-source-*` evidence. ETHOS
      support-tag publication remains with its existing issue 12.

## 8. Repository Topology, Documentation, and Deletion

- [x] 8.1 Audit `src`-equivalent Go packages, `internal/`, `cmd/`, `tools/`,
      tests, root and `.config` by semantic responsibility; replace suffix-flat or
      mixed owners with cohesive packages and remove forwarding facades.
- [x] 8.2 Reconcile tracked README, architecture, decision, operations,
      contributing and release documentation with current product behavior; all
      canonical pages must be linked, English, navigable and free of references to
      untracked prerequisites. The root index and decision register reach all
      23 canonical pages; format, links, metadata, spelling and semantic
      compression checks pass. Native link traversal proves all 23 are reachable
      from docs/README.md. Source-bound ETHOS runtime `bfd65a4e` checks all three
      family worktrees without tracked edits. Exact `ceb5948d` full proof passes
      both unchanged adopter gates, including native document quality.
      Supplemental product-profile docs-registry finds no metadata, role,
      state, duplicate, section, command, example or plan defects but demands
      four additional directory READMEs by count alone. That rule is outside
      selected adopter proof and contradicts canonical organization scenarios.
      Retain the upstream applicability defect, not marker indexes or a
      disabled gate. Evidence: `docs-current-canonical-navigation-ceb5948d.json`,
      `docs-registry-latest-runtime-c7b15070.*`,
      `runtime-refreshed-target-current-20261001.json` and
      `cold-tool-source-bound-current-proof-ceb5948d.*` under the existing
      `build/verification/supply-chain-20260930/` owner.
- [x] 8.3 Review diagrams, tables, headings, lists, examples, CLI help and
      errors for readable layout and accurate links; enforce the chosen native
      formatter/linter rather than adding one-off checks.
      The exact `ebf9adff` archive passes twenty-eight read-only help/catalogue
      invocations at widths 24, 40, 80 and 120, with no state writes or ANSI.
      `catalog --all --json` and `--json` return identical complete inventories;
      an unconfigured inventory is valid empty JSON, not an error. Table and
      paragraph boundaries, Markdown, spelling and links pass the native source
      gate. The sole Mermaid source is unchanged; its installed native render
      preserves five nodes and four labelled edges without observed clipping
      or collisions. Current packaged public-error output also passes.
      Evidence remains under `build/verification/supply-chain-20260930/`:
      `cli-ebf9adff-layout/`, `public-negative-ebf9adff/` and
      `product-concepts-ebf9adff-native.png`. Hosted Forge rendering is not
      claimed; documentation registry/navigation remains in 8.2.
      The native blank-line extension rejects single-paragraph peer separators
      while preserving complex items and literal code. Five distinguishing RED
      cases pass after repair; all 57 text-gate tests and 48 current Markdown
      files pass. Native format, policy schema, spelling, ELOC and OpenSpec
      checks pass; immutable archive bytes remain unchanged.
- [x] 8.4 Compare mature gateway, config, client and release libraries with
      retained AIGW differentiators; record one source-backed adopt/reject decision
      per candidate and delete any replaced hand-written owner.
- [ ] 8.5 Inventory obsolete branches, generated outputs, records, stale tags,
      caches and worktrees by exact owner and consumer; retire only proved
      disposable items while retaining immutable evidence and running clients.
      This lane's 2026-09-29 sweep removed obsolete ignored tmp, coverage and
      dist outputs, one superseded 0.3.3 test candidate, and four older 0.3.0
      test bundles. On October 1, the historical architecture lane's holder
      completed native retirement `66292c29`: its exact worktree, ref and lease
      are absent; signed source `01776f2f`, 3,348 raw-evidence members and eight
      analyzer members remain in the existing recovery handoff. The retained v2
      research is not acceptance; source-owned migration is required by 8.6.
      Only this terminal authoring lane remains. The current 0.3.3 candidate,
      raw verification/notarization evidence and active dependencies remain;
      obsolete tag, remote and final output retirement are still open. A later
      exact sweep retires fourteen unconsumed archive/cask payloads from two
      superseded test-signer matrices, reclaiming 55.82 MiB. All raw results,
      checksums, signatures, provenance, SBOM and license/security metadata remain
      unchanged; the fixed candidate and required predecessor are preserved.
      Seven unconsumed archive/Cask files from superseded `candidate-aba8ee13`
      are also retired, reclaiming 27.95 MiB; six metadata/signature files and
      all selected candidate/predecessor hashes are conserved. The existing
      `retired-superseded-test-matrices-20261001.json` records exact absence.
      The Fleet owner also retires the old bespoke Windows controller and its
      bytecode after proving no consumers. Its two source files remain only in
      a verified read-only tar archive; active `.py`/`.ps1` paths are absent.
      `windows-native-execution-policy-closeout-20261001.json` in the existing
      runner-capacity handoff retains the exact scope and restored host state.
      GitLab's redundant `proposal/20260926-terminal-product-convergence`
      remains separate from the existing review ref. Native retirement preview
      refuses it as not accepted; retain the object and resolve precise owned
      projection retirement at ETHOS, not by bypassing hooks.
- [x] 8.6 Migrate the Client Projection Edition Provider to the published
      Publisher v2 contract. Preserve the authored Claim Model, four reader
      questions, independent media and AIGW acceptance authority; prove exact
      package inputs, direct/declarative equivalence, relocated offline replay,
      invalid-input refusal and Git-bound rollback before deleting the v1
      materializer, captured source copies and duplicate generated identities.
      Exact alpha.7 archive and published release-manifest inputs pass native
      installed replay. Direct and declarative modes conserve the same input
      lock, selected meaning, question obligations and media; distinct closure
      metadata remains distinct. Two relocated offline consumers reproduce all
      Candidate members; tampered input fails without output and restoration
      reproduces the original Candidate. The Claim Model bytes, twelve entities,
      eleven relations/claims and four reader questions are preserved. Native
      Chrome verifies all four pages, keyboard navigation and reduced-motion
      loading without horizontal overflow. Exact predecessor replay passes
      before twenty-one superseded files are removed, with Git-bound rollback
      retained; net tracked content falls by more than 7,000 lines. Source/text
      tests pass 61 cases; native format, Markdown, links, spelling, OpenSpec,
      architecture and ELOC checks pass. Evidence: `publisher-v2-*` under the
      existing `build/verification/supply-chain-20260930/` owner. This accepts
      the source integration, not whole-product editorial scope or publication.

## 9. Frozen Source and Pre-Archive Acceptance

- [x] 9.1 Review each changed requirement against source, tests, CLI, team
      manifest, quality/CI projection and docs; remove contradictory old text and
      unused compatibility paths before freezing inputs.
      Official merged-spec review exposed inherited same-model/lexical
      fallback contradicting declared primary/alternative selection. This
      Change replaces both inherited requirements and their current prose,
      retaining every original scenario. It also closes independently found
      failure-phase counterexamples through existing result owners; focused
      regressions and the canonical native Go check pass without exemptions.
      The safe public diagnosis retains the equal-version candidate identity
      refusal; its real portable lifecycle regression passes without weakening
      current/rollback-byte preservation. All seven official delta merges are
      warning-free. Native host, client, cold-peer and final-artifact gaps remain
      explicit in their original tasks rather than becoming support claims.
- [x] 9.2 Run focused RED/GREEN suites, native static/behavior checks and strict
      official OpenSpec validation with pristine output on the frozen source; no
      skipped required gate or warning counts as pass.
      Native command dispatch selects the requested platform while Go build
      constraints select the implementation; duplicate host checks are deleted.
      The existing public Windows-target tests cover Unix identity behavior
      without a coverage-only test or suppression. Credential/projection
      siblings pass; the original per-file RED profile remains evidence.
      Historical Native Node acceptance selected Publisher alpha.1;
      current source replaces that v1 boundary through 8.6. Installed and
      ordinary source counterexamples reject stale selected owners.
      native formatter/Markdown/Mermaid behavior moves to one Node suite while
      Go retains inventory/wiring tests. All 41 JUnit cases pass without skips;
      LCOV observes all four production modules. The full local gate passes at
      95.12%. Exact `9cf23cb8` repository proof is
      `74b9431a3adbd5fb5534e417f1751349d4be80bfadddafb40172464e05797aa0`;
      native landing advances candidate/dev, not accepted dev/main.
      Both peers then expose the Windows-only path assertion; `56cdc32a` fixes
      only its expected path with `filepath.FromSlash`. GitHub run 36835345993
      and GitLab pipeline 9126 independently pass all five required jobs on
      that exact SHA, including Windows. Native watchers exit zero. Raw RED,
      source proof and peer results remain under the existing
      `build/verification/supply-chain-20260930/` owner in `native-node-owner-*`,
      `node-native-owner-exact-head-proof*` and
      `56cdc32a-{github,gitlab}-native-watch.*`. Fixed-artifact real clients,
      current-HEAD proof and distribution are separate claims.
- [ ] 9.3 Qualify one signed untagged macOS/Linux/Windows candidate with
      exact-byte install, predecessor upgrade/rollback, credentials and real-client
      journeys; retain the source and artifact hashes and disclose missing platform
      evidence. Isolated native items and environment credentials qualify this
      pre-archive candidate, not current operator-item access. Final production
      signing creates separately inventoried bytes; post-archive distribution
      and exact copied-reader authorization must pass before installed cutover.
      The existing release owner builds one six-platform 0.3.3 matrix at
      signed `9cf23cb8` in 11.709 seconds, conserving all 1,229 tracked hashes.
      `candidate-9cf23cb8` retains signed checksums, provenance, SBOM,
      license/security metadata and Cask projection; macOS signing is ad-hoc.
      Public `accept-native --candidate --baseline-tag v0.3.1 --clients`
      passes on macOS ARM64 in 320.365 seconds: lifecycle, rollback, actual
      60-second timeout, Codex 0.159.3, stable Claude 2.1.285, fourteen general
      Codex selections and a real shell tool loop. Local Hermes 0.21.5 includes
      four carried commits; official unmodified Hermes evidence stays in 4.5.
      Protected hashes are unchanged and scratch is absent; raw caller/log
      evidence is `native-current-9cf23cb8-macos*`. Preserve this candidate's
      identity: later test-only `56cdc32a` is not its provenance. Exact-candidate
      Linux real-client container proof is recorded in 5.2; current Windows
      actual-client execution is recorded in 5.3. Current final bytes,
      native-host stores, Desktop GUI, production signing and installed cutover
      remain unqualified.
      Signed source `9544b98d` rebuilds the six-platform candidate in 10.14
      seconds with all 1,229 tracked hashes conserved. Each program/archive
      equals the previously qualified `9cf23cb8` bytes; current provenance and
      full-matrix detached signature are regenerated, not borrowed. Public
      artifact admission initially rejects incorrectly selected Git-only trust
      and principal; selecting the unchanged Forge-declared artifact trust and
      signer resolves the prerequisite without widening trust. Current-source
      manifest, governance and final distribution remain separate. Exact
      archive comparisons are `9544b98d-six-platform-archive-identity-comparison.json`;
      native macOS execution and its limits are recorded in 5.4.
      The `7a5c1da6` matrix is rebuilt with the same explicit public Forge
      release-source inputs, signed checksums and current provenance. All six
      archives/programs equal `9544b98d`; the current seven-path shipped-team
      journey passes. The initial local-only build omitted those inputs; its
      mismatch, original metadata and corrected comparison are retained.
      This source/input correction is not final signing or installed cutover.
      The exact `80dbca19` matrix conserves the previously qualified six
      archives/programs while refreshing provenance. Native resource and
      published-predecessor acceptance pass; Claude, Codex, fourteen general
      Codex choices and its tool loop pass. Hermes's missing companion PATH is
      an invocation omission: the corrected four-step predecessor/candidate/
      rollback/re-upgrade journey passes in 60.401 seconds without product
      fallback. Full source proof passes in 372.608 seconds with all 1,229
      hashes unchanged. The same proposal SHA is read back on both peers;
      GitHub run 36871511315 passes all five required jobs. GitLab pipeline
      9155 exposes Linux apt timeout plus swallowed prerequisite failure, while
      macOS passes and Windows remains in cold-tool acquisition. That control
      defect is returned to the existing CUE owner; no product acceptance is
      claimed for jobs that did not reach it. Receipt/log owners are
      `native-current-candidate-80dbca19-*`,
      `native-current-hermes-80dbca19-companion-path-*`,
      `terminal-source-full-proof-80dbca19-*` and
      `80dbca19-*-current-recovered-snapshot.*` in the same verification root.
