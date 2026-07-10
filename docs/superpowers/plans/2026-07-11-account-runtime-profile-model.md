# Account Runtime Profile Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split AIGW provider identity from runtime model selection so one Account Token can back multiple selectable model Profiles.

**Architecture:** Add `Account` as the secret and endpoint owner, keep `Profile` as the user-selectable runtime model choice, and make route resolution return a resolved runtime with account, endpoint, token slot, and model. Adapters consume resolved runtime state rather than assuming profile-owned endpoints.

**Tech Stack:** Go 1.25+, Cobra, go-toml/v2, existing AIGW config/manifest/adapter packages.

## Global Constraints

- One Account has exactly one logical secret at `AIGW_TOKEN/<account-id>`.
- Runtime Profiles never own or duplicate Token values.
- Routes point to Runtime Profiles, not Accounts.
- Balance and account diagnostics operate on Accounts.
- Claude and Codex adapters must not write into each other's namespaces.
- Tokens never enter TOML, command arguments, logs, JSON output, documentation, backups, or SBOMs.

---

### Task 1: Domain model and compatibility normalization

**Files:** `internal/domain/model.go`, `internal/domain/model_test.go`, `internal/config/store.go`, `internal/config/store_test.go`

**Interfaces:** Add `Account`, profile `Account`, `Client`, `Models`, `Runtime`, and `Config.ResolveRuntime(client, explicitProfile string)`.

- [ ] Write failing tests for Account validation, profile-to-account references, client-scoped profiles, legacy profile endpoint normalization, and route resolution returning account/model.
- [ ] Run `go test ./internal/domain ./internal/config` and confirm failure is caused by missing model fields/functions.
- [ ] Implement minimal structs, normalization, validation, and runtime resolution.
- [ ] Run the same tests and require pass.

### Task 2: Manifest import/export and catalog

**Files:** `internal/manifest/manifest.go`, `internal/manifest/manifest_test.go`, `internal/catalog/catalog.go`, `internal/catalog/catalog_test.go`, `examples/team-profiles.toml`

**Interfaces:** Team manifests contain `accounts` plus `profiles`; legacy profile-only manifests import as same-id Account + Runtime Profile.

- [ ] Write failing tests for new manifest shape, secret-free export, and legacy manifest import.
- [ ] Implement parse/export/merge compatibility.
- [ ] Update built-in DMX catalog to define Account `dmx` plus Runtime Profiles for default Codex and Claude model choices.
- [ ] Run `go test ./internal/manifest ./internal/catalog`.

### Task 3: Adapter model projection

**Files:** `internal/adapters/adapter.go`, `internal/adapters/claude.go`, `internal/adapters/claude_test.go`, `internal/adapters/codex.go`, `internal/adapters/codex_test.go`, call sites in `internal/cli/*`

**Interfaces:** Adapters consume resolved runtime values or receive profile/account endpoint/model fields explicitly.

- [ ] Write failing tests proving Claude gets `ANTHROPIC_MODEL` when set and Codex provider block contains `model = "..."` when set.
- [ ] Implement minimal adapter changes.
- [ ] Run `go test ./internal/adapters ./internal/cli`.

### Task 4: CLI UX and account-scoped secret commands

**Files:** `internal/cli/daily.go`, `internal/cli/simple.go`, `internal/cli/advanced.go`, `internal/cli/wizard.go`, `internal/cli/setup.go`, `internal/cli/account_test.go`, `internal/cli/cli_test.go`, `internal/cli/simple_test.go`, `internal/cli/wizard_test.go`

**Interfaces:** `aigw use` selects Runtime Profiles; `aigw rotate` and `aigw balance` operate on Accounts; status/check show Profile and Account.

- [ ] Write failing tests for `aigw use gpt-5.5`, `aigw rotate dmx`, `aigw balance dmx`, and status output showing model/account.
- [ ] Implement minimal CLI changes while keeping legacy commands working only through normalized data, not old aliases.
- [ ] Run `go test ./internal/cli`.

### Task 5: Documentation and release proof

**Files:** `README.md`, `docs/concepts.md`, `docs/security.md`, `docs/team-rollout.md`, `docs/design/2026-07-10-aigw-cli-product-design.md`, `docs/migration.md`, `scripts/package.sh`

**Interfaces:** Docs explain Account vs Runtime Profile and model switching without increasing daily command count.

- [ ] Update docs and examples.
- [ ] Run `go test ./...`, `go vet ./...`, `sh scripts/package.sh 0.1.0-rc.1 dist-test`.
- [ ] Verify no token-like strings, no old provider-specific command residues, and release artifact architecture checks pass.
