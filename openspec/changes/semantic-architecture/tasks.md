## 1. Stage 1 — Client packages

- [x] 1.1 Add client-name confinement to `tools/architecture`. The check scans product packages for identifiers and string literals that spell an admitted client ID. A temporary policy list of the current offending packages may only shrink: the gate fails for an unlisted offender and for a listed package that no longer offends. The gate's `client_packages` report records the baseline: 19 packages name a client (Codex 13, Claude Code 13, Claude Desktop 7, Hermes 6).
- [x] 1.2 Compose the adapter registry at the composition root and pass it into the CLI application: `internal/clients` lists the built-in adapters and `cmd/aigw` injects them. `client.DefaultRegistry` and every fallback to it are deleted, a synchronizer without a registry refuses, and tests construct their registries explicitly.
- [x] 1.3 Move Codex to `internal/client/codex`: `internal/codex`, `internal/codex/catalog`, the Codex adapter, its live verification and its selection adoption. Files move with `git mv` and keep their content apart from import paths, so Git's rename detection follows them; `tools/codex/catalog` imports the new path.
- [x] 1.4 Move Claude Code (the files of `internal/claude`, the Claude adapter and its live verification) to `internal/client/claude` in the same form; `internal/claude/desktop` moves in 1.5.
- [x] 1.5 Move Claude Desktop (`internal/claude/desktop` and `internal/client/claude_desktop.go`) to `internal/client/claudedesktop` in the same form.
- [x] 1.6 Move Hermes (`internal/hermes/configuration`, the Hermes adapter and its platform verification files) to `internal/client/hermes` in the same form. The confinement policy declares the Hermes API mode `codex_responses` as a Hermes native term, which the gate accepts only as that literal in the Hermes package.
- [x] 1.7 Make the seam client-neutral. `DiscoverySource` offers only executable, file-presence and host lookups, and `Dependencies` drops `ClaudeSettingsPath`. Each adapter derives its own native paths, so the Claude path derivation leaves `internal/platform`. AIGW still refuses to start on a host where an adapter cannot derive its native paths, with the same message as before.
- [x] 1.8 Replace the client branches in `internal/cli`, `internal/synchronization`, `internal/discovery`, `internal/surface`, `internal/platform` and `internal/client/verification` with descriptor capabilities and adapter methods. `AdoptCodexSelection` and `AdoptHermesSelection` become one seam method that each adapter implements.
- [x] 1.9 Add `internal/client/contract`: one suite that every adapter factory runs against a private fixture home. The suite compares the files that Apply changes with the effects that Plan declares, requires every planned transition to happen, and requires Inspect, Verify and Plan to leave the caller's configuration unchanged. All four adapters and a test-only rehearsal adapter pass it. The rehearsal adapter lives in the composition root's tests, and its change touches only that test file.
- [x] 1.10 Shrink the confinement list to `internal/configuration` and `internal/credential`, which stage 2 deletes.
- [x] 1.11 Run the local quality graph at the stage head: every existing test passes unchanged in behavior and every coverage floor holds.
- [x] 1.12 Prove the stage head with the full ETHOS proof.
- [x] 1.13 Confirm the hosted three-platform graph and the real-client journeys on macOS, Linux and Windows pass at the stage head.

## 2. Stage 2 — Domain, store and secret

- [x] 2.1 Create `internal/domain` from the model types, validation and manifest semantics of `internal/configuration`. Each `Config` carries the `Admission` that the composition root builds from the adapters' `Spec()`, and client-specific rules become `ClientSpec` capabilities. The adapters own their client IDs and specifications, so `internal/configuration` and `internal/credential` name no client and the confinement list leaves the policy. The architecture gate admits only standard-library imports in `domain`, which names no client.
- [x] 2.2 Create `internal/store` from configuration loading, saving and schema migration, and fold `internal/transaction` into it. Both packages are gone, and the architecture gate admits `internal/store` in their place.
- [x] 2.3 Create `internal/secret` from `internal/secrets`, `internal/secrets/native` and the credential readers of `internal/credential`; the requests that prove an Account's Token move to `domain`, and the probes that send them go to `internal/secret/probe`. Both packages are gone, and the architecture gate admits `internal/secret`, `internal/secret/native` and `internal/secret/probe` in their place.
- [x] 2.4 Move the remaining client-specific code, such as Codex's model provider in `domain`, into the adapters. The Codex adapter owns its provider identifier, and no production file of `domain`, `store` or `secret` names a client.
- [x] 2.5 Remove every exported identifier in `domain`, `store` and `secret` that has no user outside its package, and make the architecture gate report any new one. The gate's `used_exports_only` rule counts the top-level identifiers of the three packages and the `secret` subpackages, with the constants of a used type as part of its API, and it reports no finding. A type-checked scan finds no exported method or field of their exported types without a user outside its package, counting an encoder that reads a tagged field.
- [x] 2.6 Run the local quality graph at the stage head.
- [x] 2.7 Prove the stage head with the full ETHOS proof.
- [x] 2.8 Confirm the hosted three-platform graph and the real-client journeys at the stage head.

## 3. Stage 3 — Command packages and acceptance scripts

- [x] 3.1 Compare `rogpeppe/go-internal/testscript` with the current CLI test harness by the code each needs for the same scripts, and record the admitted choice and its measurement in `design.md`.
- [x] 3.2 Add acceptance scripts that run the program in its own process against private homes, before any command moves, so that they hold today's behavior as the baseline: for every command its help, a representative success and a representative refusal with its exit status, and for a command that offers `--json` its JSON output on success and on refusal. A case added after the first move passes unchanged at the commit before that move. The repository gate fails for a command whose script lacks one of these.
- [x] 3.3 Move each of the 25 top-level commands into one package under `internal/cli` named after it; the root keeps the program's composition and execution: App construction, the configuration lock, help and error presentation, and group assembly. The repository gate fails for a command without its package or scripts.
- [x] 3.4 Move the use cases that several commands share out of the command packages into `internal/app`, rendering and prompts into `internal/terminal`, and the redaction rules into `internal/domain`; a command's own behavior stays in its package. The root builds every command, a command that needs capabilities receives them through the invocation context, no command package defines its own set of dependencies, and the architecture gate admits no import between command packages.
- [x] 3.5 Run the local quality graph, including the acceptance scripts, at the stage head.
- [x] 3.6 Prove the stage head with the full ETHOS proof.
- [ ] 3.7 Confirm the hosted three-platform graph runs every acceptance script and the real-client journeys pass at the stage head.

## 4. Stage 4 — Layers and tooling

- [ ] 4.1 Move program install, update, rollback and uninstall from `internal/upgrade` into `internal/program`, and the Provider-native diagnostics from `internal/providers` into `internal/provider`. Move every other top-level package under the owner of its layer, so that the layer table of task 4.3 admits each import without a per-edge allowance: `activation`, `diagnostics`, `readiness`, `renaming` and `synchronization` under `internal/app`, `process` under `internal/platform`, and `discovery` and `surface` under `internal/client`.
- [ ] 4.2 Move the release artifact format to `internal/program/artifact`. Release tooling and program update both import it, and the `artifact` package base name occurs once.
- [ ] 4.3 Add the ordered `layers` table, with at most 12 entries, to the architecture policy, and make `tools/architecture` enforce it for product packages and repository tools; delete `allowed_import_edges`. First resolve the imports that the table would refuse. The table admits a package's imports of its descendants and of its subtree's declared seams: `internal/cli/invocation` for the commands, and `internal/client` and `internal/client/verification` for the adapters. `secret` takes the file transactions from `store`, which move to a lower owner. `provider` takes diagnostic credentials and the redirect-free `probe.Send` from `secret`; sending is I/O, so it receives them through a seam instead.
- [ ] 4.4 Make the Mise `build` task write to `build/artifacts/<version>-<commit>/`; the repository root `dist/` default and its ignore entry are removed.
- [ ] 4.5 Run the local quality graph at the stage head.
- [ ] 4.6 Prove the stage head with the full ETHOS proof.
- [ ] 4.7 Confirm the hosted three-platform graph, including release construction, passes at the stage head.

## 5. Stage 5 — Names, specifications and documentation

- [ ] 5.1 Make `docs/concepts/product-concepts.md` the glossary: one canonical term per concept with its rejected synonyms. The text gate refuses a rejected synonym in code, specifications and documentation.
- [ ] 5.2 Rename identifiers, packages and prose to the canonical terms without renaming any command, so that no two packages share a base name. The architecture gate refuses a repeated base name.
- [ ] 5.3 Split `product-control-plane`, `product-quality` and `secret-storage` into capability specifications of at most 512 lines each, keeping every scenario. The documentation gate refuses a main specification above the limit.
- [ ] 5.4 Update the architecture, contributor and operations documentation to the new layout.
- [ ] 5.5 Run the local quality graph at the stage head.
- [ ] 5.6 Prove the stage head with the full ETHOS proof.
- [ ] 5.7 Confirm the hosted three-platform graph passes at the stage head.
