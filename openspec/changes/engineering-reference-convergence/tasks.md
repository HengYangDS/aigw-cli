# Tasks

## 1. Establish the authoritative baseline

- [x] 1.1 Inventory every public command, user journey, semantic package, test owner, configuration owner, documentation domain, generated projection, dependency, and delivery surface; verify every tracked carrier has one current consumer or an explicit deletion disposition.
- [x] 1.2 Reconcile all prior user feedback against canonical OpenSpec requirements; verify each item maps to one task and no copied progress ledger remains.
- [x] 1.3 Record the accepted AIGW 0.1.0 source, artifact, installation, route, credential, and client-projection baseline; verify later comparisons use immutable object and artifact identities.
- [x] 1.4 Identify all parallel implementations, forwarding wrappers, compatibility paths, stale runtime state, obsolete evidence, abandoned branches, and empty directories; verify the deletion set excludes active user, foreign-agent, and immutable historical state.

## 2. Converge configuration and onboarding journeys

- [x] 2.1 Exercise first-time interactive setup with each supported credential mode and no installed clients; verify one available Account is sufficient and no unrelated Token is required.
- [x] 2.2 Exercise `setup --from` with zero, one, and several available Accounts; verify token-free import, partial activation, and precise next actions.
- [x] 2.3 Verify deferred client installation and later synchronization for Claude Code and Codex without re-importing team configuration or changing unrelated client state.
- [x] 2.4 Reconcile `use`, client-scoped selection, default selection, `use --all`, `status`, `check`, `doctor`, `test`, and `verify`; verify defaults have one human-readable meaning and explicit client selections require no hidden global step.
- [x] 2.5 Verify setup, synchronization, selection, and repair are transactional under cancellation, output failure, concurrent external edits, and compensation failure.
- [x] 2.6 Remove obsolete setup aliases, duplicate state transitions, and unconsumed configuration fields; verify supported manifests receive explicit migration errors rather than silent reinterpretation.

## 3. Converge credential ownership and portability

- [x] 3.1 Verify automatic backend selection and explicit keyring, file, and environment modes on macOS, Linux, and Windows; confirm environment credentials work without a native credential service and remain read-only.
- [x] 3.2 Verify native Keychain, Secret Service, and Credential Manager availability checks are non-interactive, bounded, and precise; confirm denied access never causes repeated prompts or fallback weakening.
- [x] 3.3 Exercise create, read, rotate, rename, migrate, remove, rollback, and retained-command credential journeys through a published predecessor and exact candidate executable; verify exact item ownership and postimage-guarded compensation.
- [x] 3.4 Remove AIGW-installed or auto-discovered duplicate credential readers, helper identities, stale owned slots, and unsupported backend compatibility; verify default projected credential commands derive from the active installed AIGW, while an explicitly configured operator-owned command remains external and preserved.
- [x] 3.5 Review secret-bearing paths, logs, diagnostics, fixtures, release evidence, and repository history; verify no Token, private key, recoverable fragment, or secret hash enters tracked or emitted evidence.

## 4. Enforce the AIGW and external-gateway boundary

- [x] 4.1 Trace all Proxy names, loopback defaults, lifecycle assumptions, and traffic behavior in AIGW; verify AIGW retains only provider-neutral endpoint composition and no Proxy installation or runtime ownership.
- [x] 4.2 Verify direct provider endpoints and independently managed compatible endpoints through the same Account/Profile/Route contract, including absent and unavailable gateways.
- [x] 4.3 Remove mandatory Proxy-shaped defaults, duplicated compatibility behavior, and unconsumed gateway fields; verify existing supported configurations receive a precise migration path.
- [x] 4.4 Add a synthetic ordinary Provider through declarative configuration and verify no provider-specific branch enters the control-plane core.
- [x] 4.5 Add a synthetic Client through the adapter contract and verify discovery, projection, validation, rollback, disable, uninstall, and documentation without duplicating shared lifecycle logic.
- [x] 4.6 Evaluate mature lightweight libraries and frameworks against measured maintenance cost, portability, dependency depth, and displaced code; adopt only candidates with demonstrated net value and record rejected alternatives concisely.

## 5. Align logical and physical repository structure

- [x] 5.1 Build a dependency and semantic-ownership map for production packages; verify import direction, composition roots, and public interfaces match the product concepts.
- [x] 5.2 Reorganize flat suffix families, mixed-responsibility directories, concatenated names, and misplaced code into cohesive semantic packages; verify behavior remains unchanged through focused tests.
- [x] 5.3 Mirror production ownership in test organization without co-locating unrelated fixtures or creating a second implementation; verify reusable fixtures expose only stable test contracts.
- [x] 5.4 Reorganize repository tools and configuration by responsibility; verify publication, CI, quality, release, and development bootstrap each have one discoverable owner.
- [x] 5.5 Delete duplicate helpers, aliases, facades, dead branches, stale runtime artifacts, unused dependencies, orphaned evidence, and empty directories; verify no current consumer or required recovery path is lost.
- [x] 5.6 Review names, types, constants, error values, configuration keys, and CLI copy repository-wide; verify every semantic scope is narrow, consistent, English, and free of unexplained hard-coding.

## 6. Consolidate and tighten the quality graph

- [x] 6.1 Inventory every tracked file class and current quality invocation; verify format, lint, type, test, security, architecture, documentation, schema, workflow, and generated-projection coverage has no accidental gaps.
- [x] 6.2 Consolidate quality configuration into one authority per concern and one repository graph; verify GitHub and GitLab files are deterministic projections rather than copied implementations.
- [ ] 6.3 Enable the highest-value stable checks from Go, Markdown, TOML, YAML, JSON, CUE, shell, OpenSpec, spelling, links, secrets, vulnerabilities, and dependency analysis; verify each tool replaces rather than duplicates custom logic.
- [ ] 6.4 Measure executable lines, cyclomatic and cognitive complexity, nesting, parameters, test size, coverage, binary size, startup, steady-state latency, and memory; set strict but coherent thresholds with scope, rationale, and remediation.
- [ ] 6.5 Eliminate all repository-owned warnings and ambiguous skips; verify expected platform exclusions and external-capacity limits are explicit structured outcomes.
- [ ] 6.6 Add adversarial tests for invalid inputs, partial state, concurrency, cancellation, interrupted I/O, ownership drift, stale sidecars, unavailable services, and rollback failure; verify each former defect fails before its minimal repair.
- [ ] 6.7 Run focused checks after each semantic closure and the complete local graph only after prerequisites stabilize; verify no failure is hidden by retries, broad exclusions, or generated-file drift.

## 7. Upgrade and lock the development supply chain

- [ ] 7.1 Enumerate every direct runtime, build, test, documentation, CI, release, action, container, and package-manager dependency with its current stable upstream version and compatibility policy.
- [ ] 7.2 Upgrade direct dependencies in semantic batches, regenerate only canonical locks and projections, and verify each batch through focused tests before full native acceptance.
- [ ] 7.3 Verify Python 3.12 compatibility where declared while qualifying current stable Python, Go, Node, npm, mise, OpenSpec, CUE, release, security, and documentation tooling.
- [ ] 7.4 Reproduce bootstrap in a clean Work Lane on each supported host; verify repository-locked tools, local mutable environments, and content-addressed caches do not depend on ambient system versions or sibling lanes.
- [ ] 7.5 Remove superseded pins, duplicate installers, unused packages, stale caches, and abandoned generated outputs; verify lockfiles and dependency evidence describe the exact surviving graph.

## 8. Rebuild documentation and contributor experience

- [ ] 8.1 Reconfirm documentation domains and move every current document to its precise semantic owner; verify research, decisions, architecture, concepts, guides, governance, operations, and history are not mixed.
- [ ] 8.2 Rewrite the README as a concise English product entry point covering installation, first Account, explicit client activation, direct endpoint composition, diagnosis, recovery, and removal with current commands.
- [ ] 8.3 Document complete setup, deferred synchronization, credential, installation, update, rollback, uninstall, Provider extension, Client extension, and optional-gateway journeys; verify every referenced source and artifact is tracked or publicly reachable.
- [ ] 8.4 Review every heading, paragraph, list, table, code block, Mermaid diagram, internal link, and external link for semantic order, rendering, accessibility, and concise `信、达、雅` expression.
- [ ] 8.5 Create a clean-checkout contributor path from bootstrap through a bounded TDD change and review; verify a new contributor can locate the invariant, owner, test, gate, and evidence without private context.
- [ ] 8.6 Remove duplicate explanations, obsolete warnings, historical instructions presented as current, empty indexes, private-path references, and unlinked authority names; verify navigation remains complete after deletion.

## 9. Prove product and repository acceptance

- [ ] 9.1 Run the complete local quality graph at one signed clean HEAD; verify every declared gate and generated projection passes without warnings.
- [ ] 9.2 Run exact-HEAD proof and proposal review admission; verify developer and maintainer paths preserve the same object, signature, review, and branch-role semantics.
- [ ] 9.3 On macOS, Linux, and Windows, consume immutable published artifacts and verify build provenance, install, setup, credential mode, projection, update, rollback, forward recovery, uninstall, and cleanup.
- [ ] 9.4 Verify real Codex and Claude Code journeys for the selected routes where clients and credentials are available; disclose unavailable external capabilities without weakening product claims.
- [ ] 9.5 Verify GitHub and GitLab CI are projections of one graph and both observe the accepted source; confirm independent peer releases expose identical required assets when a new product version is warranted.
- [ ] 9.6 Compare performance, memory, and artifact size with the accepted baseline under equivalent inputs; resolve material regressions or record an explicit justified decision.
- [ ] 9.7 Audit every requirement and task against current source, tests, native runs, hosted runs, installed behavior, remote objects, and residue; reopen any contradicted checkbox.

## 10. Publish and cleanly close the Change

- [ ] 10.1 Publish only if product bytes changed; otherwise retain AIGW 0.1.0 and publish documentation or repository improvements through the ordinary reviewed branch path.
- [ ] 10.2 Confirm local and both Forge `dev` and `main` name the intended signed HEAD, required CI is green, and any release tag and assets retain exact identity.
- [ ] 10.3 Remove merged proposal refs, retire the Work Lane through ETHOS, and verify no active writer, lease, worktree, temporary build root, stale runtime, or empty directory remains.
- [ ] 10.4 Archive this OpenSpec Change only after all behavior, evidence, publication, and housekeeping obligations pass; verify the merged canonical specs preserve every accepted scenario.
