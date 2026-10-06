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
- [x] 2.7 Admit a selected live Account and client through endpoint and
      inference checks, while classifying absent native authorization without
      prompts or backend fallback; keep supplier-specific failures scoped to that
      Account. A focused `check --for` must not observe or charge another
      enabled client's Account.

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
- [x] 4.3 Prove that ordered existing recommendations select only usable
      unselected Routes, preserve explicit choices, and exclude manual-only
      Providers; no control-plane command claims request-time failover.
- [x] 4.4 Qualify Codex with selected non-OpenAI-family Responses-compatible
      models using its actual model chooser, authentication and tool loop; state
      native limitations instead of forging a model list.
- [ ] 4.5 Requalify Claude Code, Claude Desktop, Codex, and Hermes independently
      for native protocol, model selection, credential and rollback behavior; do not
      infer Desktop from CLI or endpoint reachability from real-client success.
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
- [x] 5.3 Run equivalent Windows native journeys, including ACL, executable
      replacement, path quoting, noninteractive environment Token and installed
      Codex/Claude consumers; report each unproved client mode explicitly.
- [x] 5.4 Run equivalent macOS journeys with native Keychain authorization and
      environment backend separately in isolated acceptance; final operator-item
      authorization remains required before post-archive installed cutover.
      No password/biometric retry loop, service restart, or hidden native-store
      policy change is permitted.
- [x] 5.5 Verify missing Codex/Claude at setup and later installation on each
      platform; deferred sync must touch only installed admitted clients and
      preserve user-owned files.
- [x] 5.6 Compare owned process, helper, temporary, journal, build and
      client-projection resources before/after success, failure, timeout and
      interruption; exact teardown preserves active installations and evidence.

## 6. Quality, Supply Chain, and Performance

- [x] 6.1 Audit every direct Go, npm, OpenSpec, Mise and release-tool version
      against the latest stable compatible upstream; update authored pins and locks
      once, then prove clean-context reproducibility and license/security
      admissibility.
- [x] 6.2 Make Go format, lint, static analysis, test, race and coverage rules
      cover product, tools and tests with zero unowned warnings; remove duplicate
      custom rules when a mature native tool proves the same property.
- [x] 6.3 Measure ELOC, logical statements, nesting and complexity by semantic
      owner; set risk-justified blocking bounds and simplify real hotspots without
      mechanical file splitting or suppressions.
- [x] 6.4 Exercise Markdown, Mermaid rendering, internal/external links, TOML,
      YAML, JSON, CUE, shell and generated-text checks on tracked content; a
      malformed or unreachable authored carrier must fail the relevant gate.
- [ ] 6.5 Verify dependency hygiene, dead-code, secrets, SBOM, vulnerabilities,
      licenses, checksums, signatures and provenance from the exact locked
      candidate; delete unconsumed parallel scanners or reports.
- [ ] 6.6 Measure startup, setup, sync, credential read, native projection,
      build and CI costs against predecessor budgets on a quiet host; preserve raw
      samples and optimize only diagnosed owners.
- [x] 6.7 Assert warnings and malformed public errors fail at their origin;
      human and JSON output must expose precise state and safe action without
      private paths, Token fragments, internal modules or tracebacks.

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
- [x] 7.3 Separate fast quality, locked bootstrap, macOS, Linux, Windows, native
      clients and release construction where independence saves time; share only
      immutable evidence and content-addressed caches. Cancel superseded review
      work only before a noninterruptible native Shell or release job starts.
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
- [ ] 7.5 Test one-peer-only and offline-local operation; an unavailable GitLab
      or GitHub must not turn the other selected peer or the local product into a
      hidden dependency, and parity claims remain peer-specific.

## 8. Repository Topology, Documentation, and Deletion

- [x] 8.1 Audit `src`-equivalent Go packages, `internal/`, `cmd/`, `tools/`,
      tests, root and `.config` by semantic responsibility; replace suffix-flat or
      mixed owners with cohesive packages and remove forwarding facades.
- [x] 8.2 Reconcile tracked README, architecture, decision, operations,
      contributing and release documentation with current product behavior; all
      canonical pages must be linked, English, navigable and free of references to
      untracked prerequisites.
- [ ] 8.3 Close CLI and document presentation, Markdown whitespace, and
      reference qualification through 8.3.1 and 8.3.2; verify native output and
      rendered layouts rather than source spelling alone.
  - [x] 8.3.1 Correct neutral Changelog headings, explicit peer history, and native repository locators without changing existing historical notes; verify strict release metadata and each actual peer destination. Keep the prepared release links distinct from unpublished tags.
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
- [x] 8.6 Migrate the Client Projection Edition Provider to the published
      Publisher v2 contract. Preserve the authored Claim Model, four reader
      questions, independent media and AIGW acceptance authority; prove exact
      package inputs, direct/declarative equivalence, relocated offline replay,
      invalid-input refusal and Git-bound rollback before deleting the v1
      materializer, captured source copies and duplicate generated identities.

## 9. Frozen Source and Pre-Archive Acceptance

- [x] 9.1 Review each changed requirement against source, tests, CLI, team
      manifest, quality/CI projection and docs; remove contradictory old text and
      unused compatibility paths before freezing inputs.
- [ ] 9.2 Run focused RED/GREEN suites, native static/behavior checks and strict
      official OpenSpec validation with pristine output on the frozen source; no
      skipped required gate or warning counts as pass.
- [ ] 9.3 Qualify one signed untagged macOS/Linux/Windows candidate with
      exact-byte install, predecessor upgrade/rollback, credentials and real-client
      journeys; retain the source and artifact hashes and disclose missing platform
      evidence.
      Production publication and installed Homebrew cutover follow the
      [Migration Plan](design.md#migration-plan). Equal product inputs do not
      rebind signatures, source epoch or provenance.
