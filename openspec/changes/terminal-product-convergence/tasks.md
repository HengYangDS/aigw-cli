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
- [ ] 2.7 Admit a selected live Account and client through endpoint and
      inference checks, while classifying absent native authorization without
      prompts or backend fallback; keep supplier-specific failures scoped to that
      Account.

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
      backend. On macOS, stage each selected Token and configured optional
      provider-diagnostic credential by explicit input in native-authorized
      items while retaining old items and captured commands; deny a capability
      claim if its item cannot be read without UI. For the one-time
      0.3.1 Homebrew link, preproject and prefetch,
      measure the bounded link gap, verify captured commands immediately and
      roll back failed candidates; disclose any residual cached-caller risk.
      Later versioned commands must remain callable throughout replacement.
      The candidate from signed source `38246801` passes the isolated macOS
      environment-backend journey from installed Homebrew 0.3.1 through upgrade,
      rollback and re-upgrade. Preprojection protects new readers in
      a simulated link gap; cached 0.3.1 callers retain the disclosed gap risk.
      The signed published 0.3.1 Linux ARM64 archive passes the corresponding
      environment-backend journey in an isolated container. GitHub run
      36563790078 at `49ba0c5ec5503b4365ff1f0c01555c05c97ca03e` passed
      the published `v0.3.1` predecessor Keychain journey on a disposable macOS
      runner, including rollback rotation and explicit restaging. Operator-item
      authorization, Linux host credential service, and Windows predecessor
      proof remain open.
- [x] 3.6 Keep an explicitly configured external credential command outside AIGW
      ownership; test its exact-path preservation, live-client verification
      boundary, and removal of only unused AIGW-owned readers.

## 4. Provider, Model, and Client Extension

- [x] 4.1 Audit Account, Model, Route, protocol, capability, and recommendation
      declarations for parallel inference or Provider-name branches; delete the
      duplicate owner and prove synthetic Provider admission.
- [ ] 4.2 Review the shipped catalogue against current upstream IDs and bounded
      live inference for DMXAPI, UCloud, and AIHubMix; retain only qualified
      models/variants and one consistent naming grammar, with source and date for
      each claim.
      The [September 30 catalogue observation](../../../docs/research/provider-model-qualification.md#provider-catalogue-and-route-evidence)
      lists all 60 then-shipped Route IDs on their Account/protocol surfaces.
      A later direct AIHubMix Chat request for `solar-pro4` returned HTTP 400
      `no_available_channel`, so that Route and its unreferenced Model were
      removed; 59 Routes remain. The catalogue listing alone did not qualify
      it. Final direct inference, client admission, and model ranking remain
      open for the retained set.
- [x] 4.3 Prove that ordered existing recommendations select only usable
      unselected Routes, preserve explicit choices, and exclude manual-only
      Providers; no control-plane command claims request-time failover.
- [x] 4.4 Qualify Codex with selected non-OpenAI-family Responses-compatible
      models using its actual model chooser, authentication and tool loop; state
      native limitations instead of forging a model list.
- [ ] 4.5 Requalify Claude Code, Claude Desktop, Codex, and Hermes independently
      for native protocol, model selection, credential and rollback behavior; do not
      infer Desktop from CLI or endpoint reachability from real-client success.
      The signed `38246801` candidate passes the isolated macOS journey with
      installed Claude Code 2.1.283, Codex 0.158.0 and Hermes 0.21.5, including
      streaming and a Codex tool loop. Claude Code and Codex pass rollback and
      re-upgrade; Hermes passes first adoption. Claude Desktop, other platforms
      and live Provider inference remain unproved.
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
      release bytes and one retained predecessor state. The signed `38246801`
      Linux ARM64 archive passes eight isolated native-product cases in the
      digest-pinned Debian Mise container, including late activation, one-of-many
      Token selection, environment and secure-file credentials, update, rollback,
      re-upgrade and uninstall. Current-setup semantics use a current-source
      fixture; lifecycle separately passes against the signed, published 0.3.1
      Linux ARM64 predecessor. At `5260160d`, the existing native system-store
      journey passed in an isolated Linux ARM64 container with a real DBus/GNOME
      Secret Service; its exact container and shallow-clone snapshot were removed.
      That source-built test does not prove published bytes, a Linux VM host, or
      real Linux clients. Native host and real-client admission remain open.
- [ ] 5.3 Run equivalent Windows native journeys, including ACL, executable
      replacement, path quoting, noninteractive environment Token and installed
      Codex/Claude consumers; report each unproved client mode explicitly.
- [ ] 5.4 Run equivalent macOS journeys with native Keychain authorization and
      environment backend separately; no password/biometric retry loop, service
      restart, or hidden native-store policy change is permitted.
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

## 6. Quality, Supply Chain, and Performance

- [ ] 6.1 Audit every direct Go, npm, OpenSpec, Mise and release-tool version
      against the latest stable compatible upstream; update authored pins and locks
      once, then prove clean-context reproducibility and license/security
      admissibility. The 2026-09-29 upstream audit found no stale direct Go,
      npm, OpenSpec, Mise-tool or release-tool pin and selected Renovate
      44.117.2 by official OCI digest. Two native lock resolutions, clean
      bootstrap and the full source gate pass; unchanged release inputs have
      accepted licenses for 43 Go and 283 npm packages. Complete cold-cache
      peer transport (7.5) and the npm release-age gate remain open.
- [x] 6.2 Make Go format, lint, static analysis, test, race and coverage rules
      cover product, tools and tests with zero unowned warnings; remove duplicate
      custom rules when a mature native tool proves the same property.
- [x] 6.3 Measure ELOC, logical statements, nesting and complexity by semantic
      owner; set risk-justified blocking bounds and simplify real hotspots without
      mechanical file splitting or suppressions.
- [x] 6.4 Exercise Markdown, Mermaid rendering, internal/external links, TOML,
      YAML, JSON, CUE, shell and generated-text checks on tracked content; a
      malformed or unreachable authored carrier must fail the relevant gate.
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
- [ ] 6.7 Assert warnings and malformed public errors fail at their origin;
      human and JSON output must expose precise state and safe action without
      private paths, Token fragments, internal modules or tracebacks.

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
- [ ] 7.4 Prove required-status enforcement, unprotected proposal branches,
      guarded merge, source-ref deletion, signer trust and branch protection on each
      selected peer without interactive authentication or divergent commit
      identities. Prove untrusted review code cannot observe persistent Shell
      runner credentials or protected-job state; retain required native evidence.
      The 2026-09-29 GitLab API reports protected macOS/Windows runners #98/#91,
      an unprotected Linux runner #86, and an unprotected source branch for MR
      !178. Its earlier green pipeline 8629 does not prove a new MR can run on
      the current runner configuration. Keep proposals unprotected; qualify
      separate disposable macOS/Windows MR executors and rerun the exact review
      SHA before claiming peer-local native review admission.
- [ ] 7.5 Test one-peer-only and offline-local operation; an unavailable GitLab
      or GitHub must not turn the other selected peer or the local product into a
      hidden dependency, and parity claims remain peer-specific.
      GitLab MR !178 pipelines 8808 and 8811 each passed quality, Linux Secret
      Service, and native Linux on their recorded review SHA; the superseded
      pipeline's two pending native jobs were canceled by exact ID. Its selected
      tools are mirrored at the GitLab peer, and the Linux image uses Mise's
      pinned Docker Hub publication. Peer-local macOS/Windows review, cold-cache
      transport, absence of GitHub credential leakage, and offline-local
      acceptance remain unproved. Transport follows the selected endpoint and
      identity, not a blanket HTTP or HTTPS assumption.

## 8. Repository Topology, Documentation, and Deletion

- [x] 8.1 Audit `src`-equivalent Go packages, `internal/`, `cmd/`, `tools/`,
      tests, root and `.config` by semantic responsibility; replace suffix-flat or
      mixed owners with cohesive packages and remove forwarding facades.
- [ ] 8.2 Reconcile tracked README, architecture, decision, operations,
      contributing and release documentation with current product behavior; all
      canonical pages must be linked, English, navigable and free of references to
      untracked prerequisites. All 23 current `docs/` pages now carry typed
      metadata; format, Markdown and spelling checks pass. ETHOS still reports
      four per-directory README gaps despite the existing root index and
      decision register. Resolve that portable rule at ETHOS rather than adding
      marker indexes; content and navigation acceptance remain open.
- [ ] 8.3 Review diagrams, tables, headings, lists, examples, CLI help and
      errors for readable layout and accurate links; enforce the chosen native
      formatter/linter rather than adding one-off checks.
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

- [ ] 9.1 Review each changed requirement against source, tests, CLI, team
      manifest, quality/CI projection and docs; remove contradictory old text and
      unused compatibility paths before freezing inputs.
- [ ] 9.2 Run focused RED/GREEN suites, native static/behavior checks and strict
      official OpenSpec validation with pristine output on the frozen source; no
      skipped required gate or warning counts as pass.
- [ ] 9.3 Qualify one signed untagged macOS/Linux/Windows candidate with
      exact-byte install, predecessor upgrade/rollback, credentials and real-client
      journeys; retain the source and artifact hashes and disclose missing platform
      evidence.
