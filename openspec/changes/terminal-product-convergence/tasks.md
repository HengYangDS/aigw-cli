## 1. Baseline and authority

- [ ] 1.1 Record exact Work Lane HEAD, tree, Lease, active Change, installed AIGW version and digest, configuration schema, selected backend, client discovery, Routes, local branches, both Forge refs, proposals, tags, Releases, CI results, and owned residue.
- [ ] 1.2 Preserve the current healthy installation, user configuration, Tokens, Codex and Claude projections, and any selected external endpoint as an immutable rollback baseline.
- [ ] 1.3 Map every tracked file and generated projection to one semantic owner, consumer, source of truth, reason to change, dependency direction, and retirement condition.
- [ ] 1.4 Map every public command, option, result, schema field, environment variable, native resource, release artifact, documentation entrypoint, and supported user journey to one product invariant and implementation owner.
- [ ] 1.5 Reconcile source, tests, canonical specs, this Change, documentation, quality policy, CUE, Forge projections, release metadata, and installed behavior; add every contradiction to this task list before implementation expands.

## 2. Natural setup and activation journey

- [ ] 2.1 Add RED acceptance for importing the reviewed team manifest with no Tokens and no clients; require successful capability import and precise deferred actions.
- [ ] 2.2 Add RED acceptance for setup with exactly one available Account Token from each supported backend; require unrelated Accounts to remain optional.
- [ ] 2.3 Add RED acceptance for one installed client, both installed clients, and neither client; require activation only where Token, Route, and client capability intersect.
- [ ] 2.4 Remove any setup validation, prompt, or loop that treats every manifest Account or both clients as mandatory.
- [ ] 2.5 Make setup human and JSON results distinguish imported Accounts and Profiles, connected Accounts, selected Routes, projected clients, and deferred continuations.
- [ ] 2.6 Prove setup failure before commit leaves configuration, credential slots, client files, and checkpoints byte-identical.
- [ ] 2.7 Prove setup commit failure compensates every AIGW-owned write and retains no temporary file, lock, partial credential, or projection.

## 3. Route, synchronization, and readiness semantics

- [x] 3.1 Add RED acceptance for separate `aigw use` operations selecting Codex and Claude, followed directly by a successful `aigw check` without a bulk or global selection.
- [ ] 3.2 Make `use` update exactly the selected Profile's declared client Route and projection; prove the other client remains byte-identical.
- [ ] 3.3 Make repeated `use` of the active Profile a no-op with no configuration, credential, checkpoint, or client-file rewrite.
- [ ] 3.4 Make `sync` discover a client installed after setup and activate its existing eligible Route without repeating setup.
- [ ] 3.5 Make `sync` observe a Token supplied after setup and activate only compatible selected or recommended Routes without changing independent user selections.
- [ ] 3.6 Remove hidden default, aggregate selection, `--all`, cross-client fallback, and duplicated route-resolution residue from runtime, migration, tests, and docs.
- [ ] 3.7 Unify `status`, `check`, and `doctor` classification of configured, deferred, ready, degraded, invalid, and unavailable state.
- [ ] 3.8 Replace journal- or implementation-shaped recovery errors with the failed user boundary, current state, and one safe next action.
- [ ] 3.9 Prove read-only commands do not mutate, start clients, open credential prompts, or read secret values except for an explicitly declared authenticated probe.

## 4. Credential portability and safety

- [ ] 4.1 Define one inspectable backend-selection result containing backend kind, availability, mutability, persistence, and recovery action without secret material.
- [ ] 4.2 Prove automatic native backend selection and value-free observation on macOS Keychain without an access prompt.
- [ ] 4.3 Prove Linux Secret Service selection on a real user bus and deterministic secure-file fallback when the bus is unavailable.
- [ ] 4.4 Prove Windows Credential Manager selection, metadata observation, persistence, replacement, and deletion from a native Windows build.
- [ ] 4.5 Prove explicit environment mode across setup, sync, check, credential helpers, and live verification; require stable operation when no native keyring exists.
- [ ] 4.6 Ensure API Tokens and Provider diagnostic credentials use distinct typed slots in one selected backend and never cross-read or substitute values.
- [ ] 4.7 Remove backend fallback searches, legacy key names, duplicate state markers, stale credential files, and damaged owned entries after migration evidence identifies no consumer.
- [ ] 4.8 Prove interruption and failure leave no partial credential or backend-selection state and do not delete unowned credentials.

## 5. Client projection and product boundary

- [ ] 5.1 Derive one admitted-client contract for discovery, configuration planning, credential binding, atomic projection, status, verification, rollback, and uninstall.
- [ ] 5.2 Make Codex CLI and Desktop use the same discovered Codex Home while preserving Desktop-only settings, sessions, models chosen by existing conversations, JSONL, and SQLite.
- [ ] 5.3 Make Claude Code projection use its official settings and credential-helper surfaces without a shell wrapper, PATH dependency, profile mutation, or plaintext Token.
- [ ] 5.4 Prove missing Codex, missing Claude, later installation, changed executable path, multiple Codex homes, malformed user configuration, and foreign managed-state conflicts.
- [ ] 5.5 Prove client projection preparation is complete before writes and multi-target failure compensates only unchanged AIGW-owned postimages.
- [ ] 5.6 Make disable and uninstall remove only AIGW-owned blocks, sidecars, helpers, checkpoints, and command projections while preserving all neighboring user state.
- [ ] 5.7 Prove direct HTTPS and an explicit loopback compatibility endpoint are ordinary interchangeable Account choices.
- [ ] 5.8 Delete every Proxy name, port, install path, lifecycle action, or implicit dependency from AIGW source, team manifests, tests, and docs unless it appears solely as a neutral composition example.

## 6. Provider and future-client extension

- [ ] 6.1 Define the minimal Provider declaration for endpoint protocols, Profiles, models, optional authentication projection, and optional diagnostic capability.
- [ ] 6.2 Add a synthetic ordinary Provider using data and shared conformance fixtures only; prove no core branch, command, installer case, or policy edit is required.
- [ ] 6.3 Model AWS Bedrock or another signing-based service as one narrow authentication Adapter only where ordinary bearer-token configuration cannot express it.
- [ ] 6.4 Define the client Adapter contract and conformance suite for future Hermes, OpenCode, Pi, Qoder, and other clients without coupling them to Provider policy.
- [ ] 6.5 Prove one representative future-client fixture can be admitted without changing existing Codex, Claude, Provider, or external-gateway behavior.
- [ ] 6.6 Evaluate maintained libraries and frameworks for configuration, secrets, rendering, update, packaging, protocol, and testing against measured deletion, dependency, security, and maintenance value.
- [ ] 6.7 Adopt only candidates with positive net value and delete the superseded custom mechanics in the same closure; record concise decisions for retained custom product semantics.

## 7. Semantic and physical topology

- [ ] 7.1 Derive a terminal package and repository map from the product ontology and one-way dependency rules before moving files.
- [ ] 7.2 Replace flat suffix families, concatenated compounds, and ambiguous buckets across `internal`, `cmd`, `tools`, tests, configuration, and docs with cohesive semantic owners.
- [ ] 7.3 Keep tests organized by behavior and evidence scope rather than mirroring incidental filenames; isolate unit, integration, native, release, and end-to-end contracts.
- [ ] 7.4 Enforce dependency direction so product packages cannot import repository tooling, tests, Forge code, ETHOS internals, or another client's implementation.
- [ ] 7.5 Remove duplicate constants, schemas, renderers, validators, state models, fixtures, wrappers, aliases, re-exports, unused dependencies, empty directories, and unreachable code.
- [ ] 7.6 Audit every root file and `.config` carrier for correct ownership and placement; relocate or delete historical residue and compatibility-oriented ignores.
- [ ] 7.7 Prove tracked entity count and duplicate ownership decrease while every required invariant retains one reachable implementation and test.

## 8. Quality authority

- [ ] 8.1 Build one positive responsibility map covering every tracked file type and semantic concern with no uncovered or multiply owned carrier.
- [ ] 8.2 Consolidate Go format, vet, static analysis, modernization, naming, documentation, error, logging, security, complexity, import, and architecture policy into comprehensible owners.
- [ ] 8.3 Adopt mature tools for dead code, dependency hygiene, vulnerabilities, secrets, licenses, SBOM, Markdown format and lint, links, TOML, YAML, JSON, CUE, Actions, and GitLab syntax where they provide net value.
- [ ] 8.4 Remove custom generic checks superseded by mature tools and retain custom checks only for explicit AIGW product or repository invariants.
- [ ] 8.5 Measure complexity, executable lines, nesting, parameters, test size, statement and branch coverage, performance, and binary size; set rational scoped thresholds with documented review conditions.
- [ ] 8.6 Require useful public and repository-tool documentation without ornamental test comments; verify generated command and configuration documentation cannot drift from source.
- [ ] 8.7 Make repository-owned warnings fatal across development, test, build, docs, packaging, native artifacts, and CI; remove all current warnings at their owners.
- [ ] 8.8 Enforce one Conventional Commit grammar through local hooks and both Forge integration paths, including generated release commits, without duplicate parsers or historical exception lists.
- [ ] 8.9 Run focused format, lint, type, architecture, dependency, security, docs, configuration, and behavior gates on each migrated owner before one final full suite.

## 9. Development environment and supply chain

- [ ] 9.1 Add minimal cross-platform `mise` tasks named `bootstrap`, `check`, `native`, and `release`; each delegates to existing ecosystem owners and no shell wrapper or parallel task runner remains.
- [ ] 9.2 Prove a fresh Work Lane reconstructs independent dependencies, build, coverage, and temporary state from committed locks while sharing only content-addressed caches.
- [ ] 9.3 Remove ambient interpreter, user-site, global mise configuration, system package, sibling checkout, and unpinned network discovery from successful local and hosted paths.
- [ ] 9.4 Audit every direct runtime, development, OpenSpec, Go, Node, mise, packaging, documentation, CI Action, and release dependency against current stable releases.
- [ ] 9.5 Advance compatible direct dependencies in their existing single sources of truth and regenerate `go.sum`, `package-lock.json`, and `mise.lock` deterministically.
- [ ] 9.6 Prove a second clean resolution is byte-clean and every tool reports the locked version on macOS, Linux, and Windows.
- [ ] 9.7 Configure one dependency-update proposal owner with release-age policy, safe grouping, vulnerability priority, automatic merge criteria, and no competing proposal on the peer Forge.
- [ ] 9.8 Generate and verify SBOM, vulnerability, license, checksum, signature, and provenance outputs without checkout paths, timestamps, credentials, or installer-local metadata.
- [ ] 9.9 Evaluate the current Go release construction against maintained alternatives only where measurements show meaningful gains; avoid replacing the native Go toolchain with a second packaging framework.

## 10. CI and dual-Forge projection

- [ ] 10.1 Define the complete semantic CI graph in CUE: fast quality, Go matrix, native source evidence, artifact construction, lifecycle acceptance, release metadata, publication, and parity.
- [ ] 10.2 Generate GitHub Actions and GitLab CI from that model; reject hand-edited drift while preserving only provider syntax and runner-capability deltas.
- [ ] 10.3 Cover proposal creation and update, review commit, maintainer fast-forward, `dev`, `main`, and tag events with required exact-commit evidence.
- [ ] 10.4 Separate independently useful jobs and parallelize safe matrices; remove monolithic verification and duplicate jobs that prove no additional fact.
- [ ] 10.5 Define exact conditions for evidence reuse and require rerun when source, platform, environment, lock, toolchain, release, or claimed fact changes.
- [ ] 10.6 Assign Windows product proof to an available native runner without requiring a symmetric but unavailable GitLab Windows runner.
- [ ] 10.7 Verify GitHub and GitLab authentication, SSH transport, author identity, commit and tag signature, protected branches, unprotected proposal branches, automatic merge, and source-branch deletion without password prompts.
- [ ] 10.8 Repair AIGW GitHub `main` and `dev` lag using the exact locally accepted signed objects; verify both Forges host identical Git object IDs and no Forge rewrites identity.
- [ ] 10.9 Keep one proposal per Change, update its exact commit, require green evidence, merge it by the admitted developer or maintainer path, and delete the proposal ref promptly.

## 11. Documentation and user experience

- [ ] 11.1 Derive documentation information architecture from audiences and journeys: evaluation, installation, first setup, daily selection, synchronization, credentials, troubleshooting, extension, contribution, release, and governance.
- [ ] 11.2 Make every directory index purposeful, remove empty or redirect-only README files, and ensure internal concepts, commands, decisions, evidence, and stable external standards use valid descriptive links.
- [ ] 11.3 Rewrite setup, team manifest, credential backend, client-later-installation, direct endpoint, optional Proxy composition, recovery, and uninstall journeys in precise natural language.
- [ ] 11.4 Generate CLI reference from the command surface or verify it against source; audit every command name, default, option, exit code, JSON field, error, and next action for narrow semantics.
- [ ] 11.5 Normalize configuration, tables, ordering, headings, examples, terminology, compounds, and visual hierarchy across code, schemas, manifests, docs, help, and generated projections.
- [ ] 11.6 Replace toy manifests with one reviewed directly consumable team profile while keeping credentials out of version control and documenting safe local overrides.
- [ ] 11.7 Remove stale claims, contradictory instructions, historical workarounds, compatibility notes without consumers, and evidence directories that do not own durable product facts.
- [ ] 11.8 Verify links, rendered Markdown, terminal layouts, narrow consoles, JSON stability, accessibility, copy-and-paste commands, and fresh-user journeys.

## 12. Cross-platform product and release evidence

- [ ] 12.1 Prove clean-room source bootstrap and the full fast gate on macOS, Linux, and Windows from the exact candidate commit.
- [ ] 12.2 Build immutable native artifacts for every declared OS and architecture; verify embedded version, checksum, signature, SBOM, provenance, startup, help, and absence of checkout dependencies.
- [ ] 12.3 Prove fresh install, status, setup, use, sync, check, doctor, update, rollback, recovery, disable, and uninstall on macOS.
- [ ] 12.4 Prove the same supported product journey on a real Linux environment, including native Secret Service and headless fallback boundaries.
- [ ] 12.5 Prove the same supported product journey on native Windows, including Credential Manager, path conventions, process environment, atomic replacement, and cleanup.
- [ ] 12.6 Prove real Codex CLI and Claude Code projections from the released artifact, including installation after setup and client invocation outside the installer shell.
- [ ] 12.7 Prove direct Provider endpoints and an explicitly selected external compatibility endpoint without giving AIGW ownership of the external service.
- [ ] 12.8 Prove update from the retained installed baseline, failed-successor rollback, repeated recovery terminality, and exact uninstall with all unowned state preserved.
- [ ] 12.9 Compare performance, memory, startup, binary size, projection latency, and credential access against explicit budgets and the retained baseline.

## 13. Destructive cleanup and terminal closeout

- [ ] 13.1 Re-run the semantic inventory and delete every obsolete implementation, migration path, schema reader, wrapper, alias, generated file, cache, runtime, temporary service, branch, tag, Release, or record with no current consumer.
- [ ] 13.2 Verify `.gitignore`, attributes, editor settings, root metadata, `.config`, OpenSpec, docs, tests, tools, and package layout contain only current intentional carriers in stable logical order.
- [ ] 13.3 Verify every user feedback item in this Change maps once to an implemented invariant and exact evidence; disclose every remaining external unknown without duplicating work.
- [ ] 13.4 Run all format, lint, type, architecture, dependency, security, documentation, configuration, behavior, native, release, and installed-runtime gates from a pristine candidate.
- [ ] 13.5 Verify signed clean candidate HEAD, exact tree, complete OpenSpec progress, current ETHOS proof, no P0 or P1 defect, and no warning or hidden skipped required gate.
- [ ] 13.6 Archive this Change through current ETHOS, land the signed candidate, align local and both Forge `main` and `dev`, publish and verify the signed tag and identical release assets, and confirm all required CI is green.
- [ ] 13.7 Delete the merged proposal, retire the Work Lane, prune obsolete local and remote refs and owned runtime residue, and verify the repository family and installed product are clean and healthy.
