# Air stale full-selection recovery implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a fail-closed, explicit `aigw route recover air` operation that removes only a verified stale AIGW full-selection residue and its mismatched fallback sidecar, then removes Air from AIGW's configured target list.

**Architecture:** Model the narrow recovery shape as a third Air-only reconciliation projection mode. The adapter owns shape validation and atomic config/sidecar mutation; the CLI owns discovery, an Air-idleness attestation, control-plane target removal, preview rendering, and rollback of the AIGW control-plane config. Route doctor calls the adapter classifier and recommends recovery only for that exact shape.

**Tech Stack:** Go 1.25.8, Cobra, existing AIGW transaction primitives, existing Go unit tests, Markdown governance checks.

## Global Constraints

- Do not modify ChatGPT/Codex conversation JSONL, SQLite, archived sessions, or model metadata.
- Do not start, stop, reload, launch, or restart Air, PyCharm, Junie, or Codex.
- `--dry-run` must not acquire the mutation lock, write config or sidecars, bind credentials, or execute a client.
- Apply requires `--confirm-host-idle`; it is an operator attestation, not a process probe.
- Recovery must not write a JetBrains `model` or `model_provider` value; it returns Air to an unselected external baseline.
- Any state other than the documented stale-full-selection/fallback-sidecar mismatch fails closed.
- Do not change GitHub plan, repository visibility, provider-native tags, or Releases.

---

### Task 1: Add an adapter-level stale-Air recovery projection

**Files:**
- Modify: `internal/adapters/codex_reconcile.go`
- Modify: `internal/adapters/codex.go`
- Modify: `internal/adapters/codex_reconcile_test.go`
- Modify: `internal/adapters/codex_inspect.go`
- Modify: `internal/adapters/codex_inspect_test.go`

**Interfaces:**
- Produces `CodexProjectionStaleAirFullSelectionRecovery` for an Air-only, desired reconciliation target.
- Produces `IsRecoverableStaleAirFullSelection(CodexInspection) bool` or an equivalent bounded adapter classifier that never exposes file contents.
- Produces `recover-stale-full-selection` and `already-external` projection plans without paths in CLI output.

- [ ] **Step 1: Write failing adapter tests for the admitted mismatch**

Create a complete AIGW full-selection configuration with `projectCodex`, then replace its full-selection sidecar with a recognized `namespaced-fallback` sidecar whose block hash is for a fallback block and whose original-provider/model fields are empty. Assert that a recovery plan has action `recover-stale-full-selection`, and that reconciliation removes the AIGW top-level selections, AIGW full block, and sidecar while retaining unrelated bytes.

```go
func TestReconcileCodexConfigsRecoversStaleAirFullSelection(t *testing.T) {
    // Arrange a valid Air authority/recovery target and a deliberately
    // mismatched, AIGW-owned fallback sidecar.
    // Plan must be recover-stale-full-selection.
    // Apply must leave no AIGW selection/block/sidecar.
}
```

- [ ] **Step 2: Run the focused adapter test and observe the expected failure**

Run:

```bash
GOTOOLCHAIN=go1.25.8 go test ./internal/adapters -run TestReconcileCodexConfigsRecoversStaleAirFullSelection -count=1
```

Expected: failure because the recovery projection mode and preparer do not yet exist.

- [ ] **Step 3: Add the recovery projection mode and exact admission predicate**

Add an Air-only projection mode constant. Permit it only for
`surface_id == "jetbrains-air-codex"` and authority `"jetbrains-ai"`.
Implement a preparer that accepts only a recognized fallback-mode sidecar,
one exact full-selection block, AIGW-marked top-level selections, no fallback
markers, no preserved original selection, and a mismatched sidecar block hash.
Use the current verified block to synthesize a legacy full-selection state,
then call the existing full-selection removal routine. Return a two-artifact
atomic plan that writes the recovered config and removes the sidecar.

```go
case CodexProjectionStaleAirFullSelectionRecovery:
    return prepareStaleAirFullSelectionRecovery(
        target.ref, configSnapshot, stateSnapshot,
    )
```

- [ ] **Step 4: Re-run the focused adapter test and verify it passes**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 5: Add failing rejection and rollback tests**

Cover: normal fallback block, full-selection sidecar, foreign/incomplete
sidecar, duplicate markers, changed provider block, preserved original
provider/model, and injected artifact-write failure. Each test asserts no
partial persistent mutation after failure.

- [ ] **Step 6: Implement minimal fail-closed validation and rollback handling**

Use existing `prepareCodexReconciliation`, `commitCodexArtifact`, and
`rollbackCodexArtifacts`; do not create a second transaction mechanism.
Keep the exact block-shape validator private to the adapters package.

- [ ] **Step 7: Verify the adapter suite**

Run:

```bash
GOTOOLCHAIN=go1.25.8 go test ./internal/adapters -count=1
```

Expected: PASS.

### Task 2: Add the explicit `route recover air` CLI command

**Files:**
- Modify: `internal/cli/advanced.go`
- Modify: `internal/cli/route_fallback.go`
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/route_fallback_test.go`

**Interfaces:**
- Adds `aigw route recover air --dry-run --json`.
- Adds `aigw route recover air --confirm-host-idle`.
- Extends `routeChangePreview` with a non-secret `configuration_action` field.

- [ ] **Step 1: Write failing CLI tests**

Build the admitted stale Air mismatch in `newAirRouteHarness`. Assert that the
JSON preview is read-only and omits the target path; apply requires
`--confirm-host-idle`; apply removes Air from AIGW's Codex target list,
does not invoke `Runner`, and removes AIGW state from the Air file. Assert the
dry-run command does not take the mutation lock and the apply command does.

```go
func TestAirRecoverDryRunIsReadOnly(t *testing.T) { /* ... */ }
func TestAirRecoverRequiresHostIdleConfirmation(t *testing.T) { /* ... */ }
func TestAirRecoverRemovesStaleFullSelectionAndTarget(t *testing.T) { /* ... */ }
```

- [ ] **Step 2: Run the focused CLI tests and observe expected failures**

Run:

```bash
GOTOOLCHAIN=go1.25.8 go test ./internal/cli -run 'TestAirRecover|TestRouteFallbackDryRunsDoNotTakeMutationLock' -count=1
```

Expected: failure because the `recover` subcommand is not registered.

- [ ] **Step 3: Register and implement the command**

Register `newRouteRecoverCommand(app)` beside fallback, restore, and doctor.
The command resolves Air, loads the control-plane config, requests the
adapter's recovery plan, and creates a preview with
`projection_mode: "none"`, action `recover-stale-full-selection`, and a
configuration action that reports whether Air is removed from the target list.
On apply it requires the idleness attestation, snapshots the AIGW config,
removes Air from Codex targets, persists the config, and calls the adapter
reconciliation. If adapter reconciliation fails, restore the captured AIGW
control-plane snapshot exactly.

- [ ] **Step 4: Extend mutation-lock classification**

Classify `route recover air --dry-run` as read-only and
`route recover air --confirm-host-idle` as mutation-capable, matching fallback
and restore behavior.

- [ ] **Step 5: Re-run focused CLI tests**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 6: Verify the complete CLI suite**

Run:

```bash
GOTOOLCHAIN=go1.25.8 go test ./internal/cli -count=1
```

Expected: PASS.

### Task 3: Improve route diagnosis and operator documentation

**Files:**
- Modify: `internal/cli/route_doctor.go`
- Modify: `internal/cli/route_doctor_test.go`
- Modify: `README.md`
- Modify: `docs/security.md`
- Modify: `docs/governance/terminal-experience-contract.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Route doctor reports `recoverable-stale-full-selection` only for the exact
  adapter classification and recommends `aigw route recover air --dry-run`.
- Existing ordinary ownership conflicts continue to recommend `aigw repair --dry-run`.

- [ ] **Step 1: Write failing route-doctor tests**

Assert the exact stale mismatch produces the new bounded state and recovery
guidance without paths, endpoints, credentials, or runner execution. Assert a
legacy PyCharm conflict still recommends generic repair, not Air recovery.

- [ ] **Step 2: Run focused route-doctor tests and observe expected failure**

Run:

```bash
GOTOOLCHAIN=go1.25.8 go test ./internal/cli -run TestRouteDoctor -count=1
```

Expected: failure because route doctor has no recoverable-mismatch branch.

- [ ] **Step 3: Implement bounded diagnostic routing**

Use the adapter's bounded classifier rather than checking configuration text in
the CLI. Keep `route doctor` observational: no credentials, native runner,
endpoint probe, session read, or mutation.

- [ ] **Step 4: Update operator-facing documents and changelog**

Document the narrow preconditions, preview/apply commands, no-fabricated-
JetBrains-selection rule, host-idleness attestation, and separate runtime UI
proof boundary. Do not say the command proves JetBrains login, billing,
endpoint routing, or a visible Air reply.

- [ ] **Step 5: Run focused documentation and route-doctor verification**

Run:

```bash
GOTOOLCHAIN=go1.25.8 go test ./internal/cli -run TestRouteDoctor -count=1
python3 scripts/check-markdown-presentation.py
python3 scripts/check-text-layout.py
```

Expected: PASS.

### Task 4: Run repository gates, commit, publish, and apply the recovery

**Files:**
- Verify only: repository quality gates, GitLab/GitHub CI, and local host state.

**Interfaces:**
- The installed AIGW is updated only through the approved release path after
  source and hosted gates pass.
- Live Air mutation happens only after a fresh dry-run, exact preimage inventory,
  and the operator's idleness attestation.

- [ ] **Step 1: Run the required local quality bundle**

Run:

```bash
GOTOOLCHAIN=go1.25.8 go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd internal tools)"
sh scripts/check-governance.sh
python3 scripts/check-markdown-presentation.py
python3 scripts/check-text-layout.py
sh scripts/test-changelog.sh
sh scripts/test-github-provider-projection.sh
sh scripts/test-github-actions-contract.sh
sh scripts/test-github-release-workflow.sh
sh scripts/test-pipeline-gates.sh
pwsh -NoProfile -File ./scripts/test-installers.ps1 -Installer ./scripts/install.ps1
```

- [ ] **Step 2: Commit the implementation**

```bash
git add internal/adapters internal/cli README.md docs/security.md \
  docs/governance/terminal-experience-contract.md CHANGELOG.md \
  docs/superpowers/specs docs/superpowers/plans
git commit -m "fix: recover stale Air full selection"
```

- [ ] **Step 3: Publish through the existing protected provider workflow**

Recheck GitLab and GitHub branch trees, run a leased GitHub dry-run, push only
the approved branch projection, never push/rewrite tags, and require both
provider CI runs to succeed before calling source publication complete.

- [ ] **Step 4: Perform the live local recovery only after fresh admission**

Run, in order:

```bash
aigw route recover air --dry-run --json
aigw route recover air --confirm-host-idle
aigw route doctor --json
aigw sync --dry-run --json
```

Before apply, capture only the permitted file identities/hashes and verify Air
is naturally idle without launching or restarting it. Report the application
configuration result separately from unproven Air UI authentication and
user-visible reply behavior.
