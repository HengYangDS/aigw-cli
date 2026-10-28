## 1. Stage 1 — Client packages

- [x] 1.1 Add client-name confinement to `tools/architecture`. The check scans product packages for identifiers and string literals that spell an admitted client ID. A temporary policy list of the current offending packages may only shrink: the gate fails for an unlisted offender and for a listed package that no longer offends. The gate's `client_packages` report records the baseline: 19 packages name a client (Codex 13, Claude Code 13, Claude Desktop 7, Hermes 6).
- [x] 1.2 Compose the adapter registry at the composition root and pass it into the CLI application: `internal/clients` lists the built-in adapters and `cmd/aigw` injects them. `client.DefaultRegistry` and every fallback to it are deleted, a synchronizer without a registry refuses, and tests construct their registries explicitly.
- [x] 1.3 Move Codex to `internal/client/codex`: `internal/codex`, `internal/codex/catalog`, the Codex adapter, its live verification and its selection adoption. Files move with `git mv` and keep their content apart from import paths, so Git's rename detection follows them; `tools/codex/catalog` imports the new path.
- [x] 1.4 Move Claude Code (the files of `internal/claude`, the Claude adapter and its live verification) to `internal/client/claude` in the same form; `internal/claude/desktop` moves in 1.5.
- [x] 1.5 Move Claude Desktop (`internal/claude/desktop` and `internal/client/claude_desktop.go`) to `internal/client/claudedesktop` in the same form.
- [x] 1.6 Move Hermes (`internal/hermes/configuration`, the Hermes adapter and its platform verification files) to `internal/client/hermes` in the same form. The confinement policy declares the Hermes API mode `codex_responses` as a Hermes native term, which the gate accepts only as that literal in the Hermes package.
- [x] 1.7 Make the seam client-neutral. `DiscoverySource` offers only executable, file-presence and host lookups, and `Dependencies` drops `ClaudeSettingsPath`. Each adapter derives its own native paths, so the Claude path derivation leaves `internal/platform`. AIGW still refuses to start on a host where an adapter cannot derive its native paths, with the same message as before.
- [x] 1.8 Replace the client branches in `internal/cli`, `internal/synchronization`, `internal/discovery`, `internal/surface`, `internal/platform` and `internal/client/verification` with descriptor capabilities and adapter methods. `AdoptCodexSelection` and `AdoptHermesSelection` become one seam method that each adapter implements.
- [x] 1.9 Add `internal/client/contract`: one suite that every adapter factory runs against a private fixture home. The suite compares the files that Apply changes with the effects that Plan declares, requires every planned transition to happen, and requires Inspect, Verify and Plan to leave the caller's configuration unchanged (Codex rejected 616a3ec9 for both gaps: feb0dd28 and eb73b750). All four adapters and a test-only rehearsal adapter pass it. The rehearsal adapter lives in the composition root's tests, and its change touches only that test file.
- [x] 1.10 Shrink the confinement list to `internal/configuration` and `internal/credential`, which stage 2 deletes.
- [ ] 1.11 Run the local quality graph at the stage head: every existing test passes unchanged in behavior and every coverage floor holds.
- [ ] 1.12 Prove the stage head with the full ETHOS proof.
- [ ] 1.13 Confirm the hosted three-platform graph and the real-client journeys on macOS, Linux and Windows pass at the stage head.

## 2. Stage 2 — Domain, store and secret

- [ ] 2.1 Create `internal/domain` from the model types, validation and manifest semantics of `internal/configuration`; the architecture gate admits only standard-library imports in `domain`.
- [ ] 2.2 Create `internal/store` from configuration loading, saving and schema migration, and fold `internal/transaction` into it.
- [ ] 2.3 Create `internal/secret` from `internal/secrets`, `internal/secrets/native` and the credential readers of `internal/credential`; Account credential validation moves to `domain`.
- [ ] 2.4 Move the remaining client-specific configuration code into the adapters and delete `internal/configuration`, `internal/credential`, `internal/secrets` and `internal/transaction`. The confinement list is then empty and is removed from the policy.
- [ ] 2.5 Remove every exported identifier in `domain`, `store` and `secret` that has no caller outside its package, and make the architecture gate report any new one.
- [ ] 2.6 Run the local quality graph at the stage head.
- [ ] 2.7 Prove the stage head with the full ETHOS proof.
- [ ] 2.8 Confirm the hosted three-platform graph and the real-client journeys at the stage head.

## 3. Stage 3 — Command packages and acceptance scripts

- [ ] 3.1 Compare `rogpeppe/go-internal/testscript` with the current CLI test harness by the code each needs for the same scripts, and record the admitted choice and its measurement in `design.md`.
- [ ] 3.2 Move each of the 25 top-level commands into one package under `internal/cli` named after it; the root keeps only group assembly.
- [ ] 3.3 Move use-case logic out of the command packages into `internal/app` and rendering, prompts and redaction into `internal/terminal`.
- [ ] 3.4 Add acceptance scripts that run the built binary against private homes for every command's help, a representative success, a representative refusal and its JSON output. The repository gate fails for a command without its package or scripts.
- [ ] 3.5 Run the local quality graph, including the acceptance scripts, at the stage head.
- [ ] 3.6 Prove the stage head with the full ETHOS proof.
- [ ] 3.7 Confirm the hosted three-platform graph runs every acceptance script and the real-client journeys pass at the stage head.

## 4. Stage 4 — Layers and tooling

- [ ] 4.1 Move program install, update, rollback and uninstall from `internal/upgrade` into `internal/program`, and the Provider-native diagnostics from `internal/providers` into `internal/provider`.
- [ ] 4.2 Move the release artifact format to `internal/program/artifact`. Release tooling and program update both import it, and the `artifact` package base name occurs once.
- [ ] 4.3 Add the ordered `layers` table, with at most 12 entries, to the architecture policy, and make `tools/architecture` enforce it for product packages and repository tools; delete `allowed_import_edges`.
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
