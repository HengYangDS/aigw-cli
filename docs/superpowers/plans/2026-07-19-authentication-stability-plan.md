# AIGW Authentication Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task.

**Goal:** Add bounded authentication stability classification to `aigw check` without changing credentials or JetBrains-owned routing.

**Architecture:** Keep `Probe` as one observation and add a deterministic `ProbeStable` aggregation layer. The CLI consumes only the aggregate result; route ownership code is reused unchanged.

**Tech Stack:** Go, `net/http`, contexts, table-driven tests, Cobra CLI.

## Global Constraints

- No GitLab or other forge access.
- No version bump, tag, Release, RC.68 mutation, or installed-binary replacement.
- No direct-upstream probe that bypasses the configured endpoint.
- No automatic token rotation or credential write.
- Tests must be written and observed failing before implementation.

---

### Task 1: Stable diagnostics state machine

**Files:**
- Modify: `internal/diagnostics/probe_test.go`
- Modify: `internal/diagnostics/probe.go`

**Interfaces:**
- Produces: `type StabilityPolicy struct { RecoveryDelays []time.Duration; AttemptTimeout time.Duration }`
- Produces: `func DefaultStabilityPolicy() StabilityPolicy`
- Produces: `func ProbeStable(context.Context, HTTPDoer, domain.Runtime, string, StabilityPolicy) Result`
- Extends: `Result` with `Attempts int` and `RecoveredTransient bool`
- Adds: `AuthenticationUnstable Kind`

- [ ] Write tests for `401,200,200,200`, `401,401,401,401`, mixed outcomes, cancellation, response closing, and token redaction.
- [ ] Run `go test ./internal/diagnostics` and verify the new tests fail because `ProbeStable` is absent.
- [ ] Implement the state machine and context-aware sleep.
- [ ] Run `go test -race ./internal/diagnostics` and verify PASS.
- [ ] Commit with `feat(diagnostics): classify transient authentication responses`.

### Task 2: CLI rendering and mutation boundary

**Files:**
- Modify: `internal/cli/simple_test.go`
- Modify: `internal/cli/simple.go`

**Interfaces:**
- Consumes: `diagnostics.ProbeStable` and `diagnostics.DefaultStabilityPolicy`.
- Produces: success rendering for recovered transient and retry-only failure rendering for unstable authentication.

- [ ] Add CLI tests for call counts, recovered success text, persistent rotate text, unstable no-rotate text, no prompt, and unchanged secret/config state.
- [ ] Run the focused CLI tests and verify RED.
- [ ] Replace the single probe call with stable probing and add bounded rendering.
- [ ] Run `go test -race ./internal/cli ./internal/diagnostics` and verify PASS.
- [ ] Commit with `feat(check): recover bounded transient authentication failures`.

### Task 3: Canonical documentation and complete gates

**Files:**
- Modify: `docs/concepts.md`
- Modify: `docs/evidence/README.md`
- Modify: `CHANGELOG.md`
- Modify: `AGENTS.md`
- Modify: `CONTRIBUTING.md`

- [ ] Document the stable 401 semantics and evidence limits.
- [ ] Add the analyzer policy: isolated non-main worktree, private TMPDIR, no auto-fix on main, owner/liveness proof before retirement.
- [ ] Add one concise Unreleased changelog entry.
- [ ] Run all repository-required gates from `CONTRIBUTING.md`.
- [ ] Commit with `docs(governance): codify bounded operational resilience`.
