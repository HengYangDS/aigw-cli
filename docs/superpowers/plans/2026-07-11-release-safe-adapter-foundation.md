# Release-safe Schema and Adapter-admission Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Claude/Codex product safely upgradeable, prove portable installation, and establish evidence-gated admission boundaries for GLM/OpenCode, Gemini, Qwen, Perplexity, and Grok without creating an unverified production route.

**Architecture:** Keep `Account -> Profile -> Route -> Adapter` canonical. Use schema v2 only for `Profile.Purpose`; v1 remains readable and an explicit upgrade changes no client projection. Future clients remain unadmitted until their isolated configuration, protocol behavior, secret handling, paid verification, and rollback are proven.

**Tech Stack:** Go 1.25, Cobra, TOML, POSIX shell, PowerShell, GitLab CI.

## Global Constraints

- No daemon, proxy, listener, automatic fallback, or desktop-client lifecycle control.
- An Account owns one Token; team manifests and client projections never carry secrets.
- Claude and Codex projections remain isolated; future adapters do not write into either directory.
- `verify` is explicit and paid; live verification is never fabricated without a Token and authorization.
- DeepSeek, Kimi, and MiniMax remain absent from templates and default routes.

---

### Task 1: Version persisted `purpose` and expose a safe upgrade

**Files:**
- Modify: `internal/domain/model.go`, `internal/config/store.go`, `internal/manifest/manifest.go`
- Modify: `internal/cli/advanced.go`, `internal/cli/app.go`
- Test: `internal/domain/model_test.go`, `internal/manifest/manifest_test.go`, `internal/cli/advanced_test.go`
- Modify: `README.md`, `docs/team-rollout.md`, `examples/team-profiles.toml`

**Interfaces:**
- `domain.CurrentConfigVersion == 2`.
- `Config.NeedsUpgrade() bool` detects a readable v1 configuration.
- `Config.Upgrade() bool` changes only the schema version.
- `aigw config upgrade` writes the upgrade atomically and never syncs, authenticates, starts, or stops a client.

- [ ] Write failing tests for v1 readability, v1-with-purpose rejection, v2 validation, manifest v1/v2 rules, and `config upgrade` leaving Codex target bytes and Runner calls unchanged.
- [ ] Run focused tests and observe missing API failures.
- [ ] Implement `LegacyConfigVersion`, `CurrentConfigVersion`, `NeedsUpgrade`, and `Upgrade`; keep v1 readable, allow v2, and reject purpose under v1.
- [ ] Make `config.Store.Save` write v2 after an explicit upgrade or any purpose-bearing mutation; preserve atomic backup semantics.
- [ ] Make manifest v1 reject purpose, manifest v2 accept it, and exports emit v2 when required.
- [ ] Implement `aigw config upgrade`, lock it as a mutation, and document upgrading before importing the v2 team template.
- [ ] Verify: `go test ./internal/domain ./internal/config ./internal/manifest ./internal/cli -count=1`, then `go test -race ./...` and `go vet ./...`.
- [ ] Commit: `feat: add explicit config schema upgrades`.

### Task 2: Add hermetic portable-install coverage

**Files:**
- Create: `scripts/test-portable-install.sh`
- Modify: `scripts/install.sh`, `.gitlab-ci.yml`, `README.md`

**Interfaces:**
- `scripts/test-portable-install.sh <binary>` creates a temporary home, invokes the installer with a local binary, proves `aigw --version`, proves uninstall is ownership-scoped, and removes all temporary state.
- `AIGW_SOURCE_BINARY` is an explicit test-only local input; downloaded releases retain authenticated checksum validation.

- [ ] Write the smoke script first; it must fail because the installer has no local source seam.
- [ ] Run `go build -o /tmp/aigw-install-smoke ./cmd/aigw && sh scripts/test-portable-install.sh /tmp/aigw-install-smoke` and confirm that failure.
- [ ] Implement executable validation and the `AIGW_SOURCE_BINARY` branch before adjacent-binary and release-download paths.
- [ ] Verify shell syntax, smoke lifecycle, source binary preservation, and temporary-home removal.
- [ ] Add the build + smoke test to CI `verify`; commit `test: cover portable installation lifecycle`.

### Task 3: Make Adapter admission an evidence-gated product contract

**Files:**
- Create: `docs/adapter-admission.md`
- Modify: `docs/model-strategy.md`, `README.md`, `docs/team-rollout.md`

**Acceptance record for every Adapter:** executable/version, exclusive config boundary, environment keys, endpoint protocol, model selector behavior, streaming/tool tests, secret-isolation proof, cleanup proof, one explicit paid verification, and rollback evidence.

- [ ] Record official facts: Z.AI exposes Anthropic for Claude Code and a coding PaaS endpoint for OpenCode; Gemini CLI uses a process `GEMINI_API_KEY`; Qwen Code model providers use `envKey`; OpenCode supports `OPENCODE_CONFIG_DIR` and `{env:NAME}`; Perplexity offers a Responses-compatible Agent API.
- [ ] State GLM as a separate provider Account candidate on supported Claude/OpenCode protocol boundaries, not a synthetic shared `glm` client.
- [ ] Require unadmitted clients to remain unavailable in normal `profile add`, routing, and templates.
- [ ] Run `sh scripts/check-retired-residue.sh` and a local Markdown link check; commit `docs: define adapter admission evidence gates`.

### Task 4: Harden release preflight without inventing external state

**Files:**
- Create: `docs/release-readiness.md`
- Modify: `.gitlab-ci.yml`, `README.md`

- [ ] Inspect local signing identity availability and GitLab reachability once with bounded commands, recording status only.
- [ ] Add a tagged-release guard that checks required protected signing/notarization variable names without printing their values; non-tag package verification remains usable.
- [ ] Record signing requirements, GitLab runner requirements, remote recovery commands, and the distinction between local package proof and remote release proof.
- [ ] Verify `sh scripts/package.sh 0.1.0-rc.1 /tmp/aigw-release-proof`, artifact count/checks, and removal of temporary output.
- [ ] Commit `ci: fail closed on release prerequisites`.

### Task 5: Close locally verifiable work and attempt remote publication once

**Files:**
- Modify: `CHANGELOG.md`, `README.md`

- [ ] Run `go test -race ./...`, `go vet ./...`, retired-residue checks, install smoke, full package matrix, artifact checks, and `git diff --check`.
- [ ] Record separately: locally complete work versus external blockers (signing identity, GitLab recovery, push/MR/main merge/release, independent provider Token, explicit paid verification).
- [ ] Commit `docs: record release readiness evidence`.
- [ ] Attempt one `git push -u origin codex/initial-product`. If unavailable, preserve the clean committed branch and report the exact blocker without claiming push, merge, release, signing, or live-provider verification.
