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

- [x] Write failing tests for v1 readability, v1-with-purpose rejection, v2 validation, manifest v1/v2 rules, and `config upgrade` leaving Codex target bytes and Runner calls unchanged.
- [x] Run focused tests and observe the missing API failure.
- [x] Implement `LegacyConfigVersion`, `CurrentConfigVersion`, `NeedsUpgrade`, and `Upgrade`; keep v1 readable, allow v2, and reject purpose under v1.
- [x] Preserve v1 on ordinary saves; require explicit upgrade before a purpose-bearing mutation so older files are never silently made incompatible. Atomic backup semantics remain unchanged.
- [x] Make manifest v1 reject purpose, manifest v2 accept it, and exports emit v2 when required.
- [x] Implement `aigw config upgrade`, lock it as a mutation, and document upgrading before importing the v2 team template.
- [x] Verify focused and full race/vet suites.
- [x] Commit: `feat: add explicit config schema upgrades`.

### Task 2: Add hermetic portable-install coverage

**Files:**
- Create: `scripts/test-portable-install.sh`
- Modify: `scripts/install.sh`, `.gitlab-ci.yml`, `README.md`

**Interfaces:**
- `scripts/test-portable-install.sh <binary>` creates a temporary home, invokes the installer with a local binary, proves `aigw --version`, proves uninstall is ownership-scoped, and removes all temporary state.
- `AIGW_SOURCE_BINARY` is an explicit test-only local input; downloaded releases retain authenticated checksum validation.

- [x] Write the smoke script first; it fails before the installer has a local source seam.
- [x] Run the smoke script against a locally built binary and confirm that failure.
- [x] Implement executable validation and the `AIGW_SOURCE_BINARY` branch before adjacent-binary and release-download paths.
- [x] Verify shell syntax, smoke lifecycle, source binary preservation, and temporary-home removal.
- [x] Add the build + smoke test to CI `verify`; commit `test: cover portable installation lifecycle`.

### Task 3: Make Adapter admission an evidence-gated product contract

**Files:**
- Create: `docs/adapter-admission.md`
- Modify: `docs/model-strategy.md`, `README.md`, `docs/team-rollout.md`

**Acceptance record for every Adapter:** executable/version, exclusive config boundary, environment keys, endpoint protocol, model selector behavior, streaming/tool tests, secret-isolation proof, cleanup proof, one explicit paid verification, and rollback evidence.

- [x] Record official facts: Z.AI exposes Anthropic for Claude Code and a coding PaaS endpoint for OpenCode; Gemini CLI uses a process `GEMINI_API_KEY`; Qwen Code model providers use `envKey`; OpenCode supports `OPENCODE_CONFIG_DIR` and `{env:NAME}`; Perplexity offers a Responses-compatible Agent API.
- [x] State GLM as a separate provider Account candidate on supported Claude/OpenCode protocol boundaries, not a synthetic shared `glm` client.
- [x] Require unadmitted clients to remain unavailable in normal `profile add`, routing, and templates.
- [x] Run retired-residue and local Markdown link checks; commit `docs: define adapter admission evidence gates`.

### Task 4: Harden release preflight without inventing external state

**Files:**
- Create: `docs/release-readiness.md`
- Modify: `.gitlab-ci.yml`, `README.md`

- [x] Inspect local signing identity availability and GitLab reachability once with bounded commands, recording status only.
- [x] Add a GA-tag gate that fail-closes until protected macOS/Windows signing and notarization jobs are materially implemented; non-tag and RC package verification remain usable.
- [x] Record signing requirements, GitLab runner requirements, remote recovery commands, and the distinction between local package proof and remote release proof.
- [x] Verify the full RC package matrix, artifact count/checks, and removal of temporary output.
- [x] Commit `ci: fail closed on release prerequisites`.

### Task 5: Close locally verifiable work and attempt remote publication once

**Files:**
- Modify: `CHANGELOG.md`, `README.md`

- [x] Run `go test -race ./...`, `go vet ./...`, retired-residue checks, schema and install smoke, full package matrix, artifact checks, documentation link checks, and `git diff --check`.
- [x] Record separately: locally complete work versus external blockers (signing identity, GitLab recovery, push/MR/main merge/release, independent provider Token, explicit paid verification).
- [ ] Commit `docs: record release readiness evidence`.
- [ ] Attempt one `git push -u origin codex/initial-product`. If unavailable, preserve the clean committed branch and report the exact blocker without claiming push, merge, release, signing, or live-provider verification.
