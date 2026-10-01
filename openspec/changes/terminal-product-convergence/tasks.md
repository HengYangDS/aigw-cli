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
      as item authorization. The post-archive installed transition must stage
      and verify each selected operator Token and configured diagnostic item
      through the exact final copied reader before any projection or link cutover;
      retain old items and original commands. The one-time 0.3.1 Homebrew link
      transition must preproject, prefetch, measure its bounded link gap, verify
      captured commands immediately and restore the predecessor on failure.
      Disclose residual cached-caller risk; never claim denied access as ready.
      Later versioned commands must remain callable throughout replacement.
      At signed source `cdfc0bf6`, the isolated macOS environment-backend
      journey stages actual Homebrew 0.3.1 bytes and passes preprojection,
      simulated link gap, upgrade, rollback, re-upgrade and uninstall. New
      readers survive the gap; cached 0.3.1 callers retain the disclosed risk.
      The signed published 0.3.1 Linux ARM64 archive passes the corresponding
      environment-backend journey in an isolated container. GitHub run
      36563790078 at `49ba0c5ec5503b4365ff1f0c01555c05c97ca03e` passed
      the published `v0.3.1` predecessor Keychain journey on a disposable macOS
      runner, including rollback rotation and explicit restaging. Operator-item
      authorization, Linux host credential service, and Windows predecessor
      proof remain open.
      The signed `2c7fbe89` Linux ARM64 candidate also passes authentic 0.3.1
      succession with an isolated real DBus/GNOME Secret Service as UID 1000.
      Automatic backend selection is keyring; old and new commands remain
      callable through upgrade, rollback and re-upgrade, and uninstall retains
      the stored Token. Linux host and Windows evidence remain separate.
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
      The [September 30 catalogue observation](../../../docs/research/provider-model-qualification.md#provider-catalogue-and-route-evidence)
      lists all 60 then-shipped Route IDs on their Account/protocol surfaces.
      A later direct AIHubMix Chat request for `solar-pro4` returned HTTP 400
      `no_available_channel`, so that Route and its unreferenced Model were
      removed; 57 Routes remain after replacing three GPT-6 Sol Routes with
      AIHubMix, DMXAPI, and UCloud GPT-6.1 Sol Routes and withdrawing the Fable 5.1 CC
      channel after three bounded requests failed to complete. Direct DMXAPI
      text and strict function-call probes plus two isolated Codex tool loops now
      support the Codex recommendation. Official Hermes v0.21.5 source also
      completed two direct DMXAPI 6.1 Sol turns, and an isolated AIGW `verify`
      completed after staging its versioned reader. Later UCloud 6.1 Sol
      inference and installed Codex/Hermes `verify` sessions also passed.
      A catalogue listing alone did not qualify either client. All 57 retained
      exact Account/wire/protocol contracts have dated completed inference and
      at least one compatible native-client observation. The October 1
      [curated choices](../../../docs/research/provider-model-qualification.md#curated-model-choices)
      review supplies the Gemini, GLM, Kimi and Qwen primary-source rationale;
      vendor positioning is not an independent global ranking. Low-level import
      now accepts an explicit set of
      obsolete Routes in the same guarded commit as the incoming catalogue;
      focused tests preserve selected Routes and shared or incoming Models,
      reject invalid selectors before writing, and reject dangling retained
      recommendations. October 1 metadata-only import installed 26 Models and
      57 Routes, retired twelve obsolete unselected Routes, and preserved
      Accounts, explicit bindings and credential commands. The historical
      reader's Hermes menu projection failed exact wire-ID acceptance and was
      fully rolled back, preserving all sixteen protected file identities.
      The current native team journey checks delivered wire IDs, channel
      variants and unowned settings; final-artifact host synchronization remains
      open.
      Public Route addition now distinguishes canonical `--model` from an
      optional exact `--upstream-model`; catalogue continuation supplies the
      required `--protocol`. RED/GREEN and sibling tests preserve existing
      Model identities, Routes and Client selections without alias inference.
      A later full-manifest inference audit rejected only AIHubMix Fable 5.1
      with an explicit temporary model-unavailable HTTP 400. That Route is
      withdrawn from the shipped catalogue; DMXAPI and UCloud Fable remain.
      The diagnostic regression distinguishes that refusal from malformed
      model requests and preserves one bounded request with no auth retry.
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
      Windows/Linux real clients, native stores,
      Desktop GUI, installed-host cutover and final signed bytes remain open.
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
- [ ] 5.3 Run equivalent Windows native journeys, including ACL, executable
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
      rerun. Real clients, Credential Manager, performance and arbitrary-code
      review containment remain unqualified.

- [ ] 5.4 Run equivalent macOS journeys with native Keychain authorization and
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
- [ ] 5.6 Compare owned process, helper, temporary, journal, build and
      client-projection resources before/after success, failure, timeout and
      interruption; exact teardown preserves active installations and evidence.
      Focused transaction regressions cover cancellation and reader loss
      after projection admission; test cleanup has its own bounded context.
      Exact `ebf9adff` public Hermes verification passes five abnormal-return
      cases on macOS and Linux ARM64 container: success, client failure,
      parent exit, active SIGTERM and the actual 60-second protocol deadline.
      Each preserves private config, projection, reader and program inventories,
      removes the verification home and owned descendants, and preserves an
      unrelated process. Both independent packaged-abnormal receipts record
      reclaimed scratch and unchanged candidate bytes. These synthetic-client
      environment-backend results do not prove Windows, native Linux host,
      real-client or final signed-artifact acceptance; those scopes remain open.
      The same five-case contract now has one portable tracked release fixture,
      including native Windows console cancellation and exact process handles.
      The existing native acceptance owner invokes it with the chosen candidate;
      canonical lint includes its build tag. The fixture passes on exact macOS
      candidate bytes and compiles for Windows ARM64. Full source quality and
      strict coverage pass without changing the 500-line limit. Windows native
      execution and final peer qualification remain required.
      Signed `bf35796c` also repairs Windows-selected lint before native
      execution; all three OS selections and the complete source gate pass.
      Its exact-source ARM64 fleet pilot passes success, failure, parent-exit,
      the actual 60.37-second deadline and assertion cleanup. Interrupt alone
      originally fails with sender exit `0xc000013a`. Signed `3e68e5e7`
      waits for the console-event acknowledgement before checked cleanup;
      its exact-source interrupt-only native successor passes in 0.56 seconds
      with unchanged candidate and resource inventory. The original RED and
      actual native GREEN remain distinct receipts, not an all-five-case
      final-source claim. The executor's empty-cache cleanup prerequisite was
      separately repaired; all exact owned children and processes are absent,
      with original pauses, isolation and service identity preserved. Final
      native-source, store, client and distribution qualification remain open.
      The `90e13374` native Linux host run also passes the complete five-case
      contract against unchanged `ebf9adff` bytes, with exact owned resources
      reclaimed. Windows final-source, real-client and native-store scope is
      not inferred from this result.

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
- [ ] 7.3 Separate fast quality, locked bootstrap, macOS, Linux, Windows, native
      clients and release construction where independence saves time; share only
      immutable evidence and content-addressed caches. Cancel superseded review
      work only before a noninterruptible native Shell or release job starts.
      Native CI forwards explicit candidate, predecessor and client inputs
      to one release owner for peer downloads, trust, extraction and cleanup;
      duplicate GitHub download and Keychain selectors are deleted. Snapshot
      construction and native journey children receive no Forge credential
      environment. Signed two-peer fixtures, input rejection, forwarding,
      child-deadline and current-manifest wire-menu regressions pass;
      fixed-artifact execution with real clients on both peers remains required.
      Signed docs source `90908d5d` passes GitLab pipeline 9049 and GitHub
      run 36769384552 with all five required jobs on each peer; these source
      results do not qualify the fixed candidate's Windows/client journey.
      Signed `b8e56dfc` closes the October 1 synthetic-only reproduction of job-netrc
      pointer and Mise credential inheritance despite cleared Token variables.
      One override owner now scopes construction and acceptance without
      changing parent acquisition authorization. Both subject paths reject
      inherited carriers while the selected acquisition path retains them;
      this does not prove arbitrary-code Runner containment or hosted cold-peer
      execution. Its full source gate passes at 95.09% with 1,228 hashes conserved.
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
      The GitLab registry reports all 45 current ARM64 GitHub tool assets with
      lock-matching SHA-256; CUE projects the job-scoped mirror and Linux uses
      Mise's pinned Docker Hub image. Real Job Token downloads, cold-cache
      peer-outage behavior without GitHub fallback or credential leakage,
      and full cold-cache execution remain unproved. Existing mirrored-provenance
      acceptance passed without network or inherited authorization, and local
      matrix fixtures passed without Forge downloads. Those bounded proofs do
      not replace the real Job Token tool graph. Transport follows the selected
      endpoint and identity, not a blanket HTTP/HTTPS assumption.
      Reciprocal glab transport now projects browser and encoded API URLs from
      the same CUE owner to GitHub-local assets. Every locked platform URL and
      consuming workflow passes focused regression; native cold macOS install,
      API-only download, missing-copy refusal and checksum-tamper refusal pass
      with external network denied and no authorization or cookies. The six
      upstream archives match `mise.lock`; source tests do not establish their
      hosted publication or the complete three-platform cold graph.

## 8. Repository Topology, Documentation, and Deletion

- [x] 8.1 Audit `src`-equivalent Go packages, `internal/`, `cmd/`, `tools/`,
      tests, root and `.config` by semantic responsibility; replace suffix-flat or
      mixed owners with cohesive packages and remove forwarding facades.
- [ ] 8.2 Reconcile tracked README, architecture, decision, operations,
      contributing and release documentation with current product behavior; all
      canonical pages must be linked, English, navigable and free of references to
      untracked prerequisites. The root index and decision register reach all
      23 canonical pages; format, links, metadata, spelling and semantic
      compression checks pass. At `54b815ae`, independent read-only execution
      of the bound ETHOS native docs-registry provider finds no metadata, role,
      state, duplicate, section, command, example, plan or length defects. Its
      only four findings require per-directory READMEs for architecture,
      decisions, governance and research despite that canonical navigation.
      Resolve applicability at the ETHOS rule owner rather than adding marker
      indexes; formal qualified-runtime adoption and registry acceptance remain
      open. Historical long-document failures remain evidence, not current
      findings.

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
- [x] 8.4 Compare mature gateway, config, client and release libraries with
      retained AIGW differentiators; record one source-backed adopt/reject decision
      per candidate and delete any replaced hand-written owner.
- [ ] 8.5 Inventory obsolete branches, generated outputs, records, stale tags,
      caches and worktrees by exact owner and consumer; retire only proved
      disposable items while retaining immutable evidence and running clients.
      This lane's 2026-09-29 sweep removed obsolete ignored tmp, coverage and
      dist outputs, one superseded 0.3.3 test candidate, and four older 0.3.0
      test bundles. The current 0.3.3 candidate, raw verification/notarization
      evidence, active development dependencies and foreign lanes remain;
      branch, tag, remote and cross-worktree cleanup remains open.

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
      Signed source `ebf9adff` passes the full local gate at 95.09% and the
      independent Linux native race/coverage graph at 95.02% with all sixty
      canonical packages observed. Seven distinguishing credential mutations
      fail; the restored package passes. Official OpenSpec checks eleven items
      with zero findings. GitLab pipeline 9074 and GitHub run 36790679136,
      attempt 1, independently pass all five required source/native jobs at
      that exact commit. Final artifact and release claims remain in 9.3 and
      the post-archive release sequence.
      Both peers later pass all five required jobs at signed `6b075f28`.
      Signed `f95db769` scopes source fixtures to selected native journeys and
      their child test owner. Exact-candidate selection and failure-ownership
      RED/GREEN, three-platform static checks and one full source gate pass;
      1,228 tracked hashes are conserved and coverage remains 95.09%.
      Its hosted successor and native-store qualification remain separate.
      Both peers pass all five required jobs at signed `ea8d5d8f`: GitLab
      pipeline 9101 and GitHub run 36813731239. Later source changes require
      their own review result; fixed-candidate native gaps remain open.
- [ ] 9.3 Qualify one signed untagged macOS/Linux/Windows candidate with
      exact-byte install, predecessor upgrade/rollback, credentials and real-client
      journeys; retain the source and artifact hashes and disclose missing platform
      evidence. Isolated native items and environment credentials qualify this
      pre-archive candidate, not current operator-item access. Final production
      signing creates separately inventoried bytes; post-archive distribution
      and exact copied-reader authorization must pass before installed cutover.
