# Air Host-Mirror and Orphan Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Distinguish a JetBrains-owned Air mirror from a true exact orphan,
add secret-free runtime attestation, and recover an admitted orphan through a
case-bound ledger, quarantine, and settlement workflow for RC.67.

**Architecture:** The adapters package owns exact managed-projection parsing,
fingerprinting, two-surface classification, and the in-memory orphan cleaner.
New attestation and recovery packages own bounded Air-log evidence and the
hashes-only recovery journal. The CLI composes those packages without reading
credentials or locking for read-only operations.

**Tech Stack:** Go 1.25.12, Cobra, `net/url`, SHA-256, existing transaction
snapshots, JSON state, Go tests, and repository shell/Python gates.

## Global Constraints

- Air remains JetBrains AI authority. Never edit or read conversation JSONL,
  SQLite, archived sessions, model metadata, prompts, or responses.
- Never start, stop, reload, restart, authenticate, or process-probe Air.
- Never emit a path, URL, raw host/port, endpoint, model, PID, CallTraceId,
  header, body, token, prompt, response, or quarantine content.
- `external-host-mirror` requires positive parity with a recognized current
  standalone full-selection projection.
- Expose a true orphan as `orphaned-exact-full-selection`; the generic internal
  `orphaned-aigw-marker` name must not leak through the Air-aware contract.
- Expose `host-mirror-runtime-attested` only for a fresh JetBrains-only log
  generation whose configuration state is `external-host-mirror`.
- Attestation and all dry-runs are credential-free, write-free,
  directory-creation-free, and mutation-lock-free.
- Recover apply requires exact `--case-id`, `--confirm-host-idle`, and
  `--ack-unset-external-selection`. There is no `--force`.
- Recovery writes no replacement selection. It leaves an unset external
  baseline and waits for a separately observed host roundtrip.
- Quarantine is private storage, not a command.
- Writes use captured preimages, guarded commits, reverse compensation,
  `0700` directories, and `0600` files.
- Keep `internal/cli/root.go` at `0.1.0-dev`, Go at 1.25.12, and current forge
  and signer manifests unchanged.

---

### Task 1: Add exact Air mirror/orphan classification and cleaning

**Files:**
- Create: `internal/adapters/codex_air.go`
- Test: `internal/adapters/codex_air_test.go`

**Interfaces:**

```go
const (
    AirStateExternalHostMirror = "external-host-mirror"
    AirStateOrphanedExactFullSelection = "orphaned-exact-full-selection"
)

type AirOrphanRemovalPlan struct {
    Preimage transaction.FileSnapshot
    Cleaned transaction.FileSnapshot
    ProjectionFingerprintSHA256 string
}

func InspectAirCodexConfig(airPath, standalonePath string) (CodexInspection, error)
func PlanAirOrphanRemoval(airPath, standalonePath string) (AirOrphanRemovalPlan, error)
```

- [ ] **Step 1: Write failing classification tests**

Add `TestInspectAirCodexConfigClassifiesExactReferenceMirror`,
`TestInspectAirCodexConfigClassifiesExactOrphan`, and
`TestPlanAirOrphanRemovalPreservesUnrelatedBytes`. The mirror fixture has a
recognized standalone full-selection sidecar and different unrelated Air bytes
but the same canonical projection fingerprint. The orphan changes one managed
model or endpoint byte and must expose `orphaned-exact-full-selection`.

- [ ] **Step 2: Run the failing tests**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/adapters -run 'TestInspectAirCodexConfig|TestPlanAirOrphanRemoval' -count=1
```

Expected: FAIL because the Air-aware APIs do not exist.

- [ ] **Step 3: Implement the exact fingerprint**

Add private helpers:

```go
func exactAirManagedProjection(text string) (*airManagedProjection, bool)
func normalizeAirProjectionNewlines(text string) string
func normalizeAirProjectionLine(line string) string
```

Hash the design's versioned provider line, optional model line, and exact block
after CRLF-to-LF normalization only. Require one provider selection, at most one
managed model, one exact block, no fallback, no duplicate, and no partial or
unmanaged AIGW content. Require recognized, hash-matching standalone attribution
before returning `external-host-mirror`.

- [ ] **Step 4: Implement the in-memory cleaner**

Reuse the existing exact full-selection removal logic after shape validation.
Return cleaned bytes only for `orphaned-exact-full-selection`; write no sidecar
and no provider/model replacement.

- [ ] **Step 5: Add fail-closed table tests**

Cover CRLF parity, duplicate selection/model/block, partial markers, field-order
change, fallback content, foreign/invalid Air sidecar, missing standalone,
unrecognized standalone sidecar, and mismatched standalone block hash.

- [ ] **Step 6: Verify and commit**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/adapters -count=1
git add internal/adapters/codex_air.go internal/adapters/codex_air_test.go
git commit -m "feat: classify Air host mirrors and exact orphans"
```

---

### Task 2: Route doctor must expose the bounded Air states

**Files:**
- Modify: `internal/cli/route_doctor.go`
- Test: `internal/cli/route_doctor_test.go`

**Interfaces:**

```go
ConfigurationState string `json:"configuration_state,omitempty"`
RecoveryState string `json:"recovery_state,omitempty"`
```

- [ ] **Step 1: Write failing doctor tests**

Add `TestRouteDoctorTreatsExactAirHostMirrorAsExternal`,
`TestRouteDoctorReportsOrphanedExactFullSelection`, and
`TestRouteDoctorKeepsPartialAirResidueFailClosed`. A mirror keeps `report.OK`
true and has no mutation guidance. An exact orphan returns only
`aigw route recover-orphan air --dry-run --json` as the human next action.

- [ ] **Step 2: Run the focused tests**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/cli -run 'TestRouteDoctorTreatsExactAirHostMirror|TestRouteDoctorReportsOrphaned|TestRouteDoctorKeepsPartial' -count=1
```

Expected: FAIL because doctor still uses generic single-file inspection.

- [ ] **Step 3: Integrate `InspectAirCodexConfig`**

Resolve the canonical standalone surface once. Preserve ADR-0003 guidance.
Use next-action precedence: stale-sidecar recovery, exact-orphan recovery, then
ordinary repair. Never mark an external mirror as an AIGW-managed Air target.

- [ ] **Step 4: Verify and commit**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/cli -run TestRouteDoctor -count=1
git add internal/cli/route_doctor.go internal/cli/route_doctor_test.go
git commit -m "fix: distinguish Air mirrors from exact orphans"
```

---

### Task 3: Add bounded Air-log attestation primitives

**Files:**
- Create: `internal/attestation/air.go`
- Test: `internal/attestation/air_test.go`
- Modify: `internal/platform/paths.go`
- Test: `internal/platform/paths_test.go`

**Interfaces:**

```go
type AirRuntimeAttestation struct {
    SurfaceID string `json:"surface_id"`
    ConfigurationState string `json:"configuration_state"`
    State string `json:"state"`
    RuntimeAuthority string `json:"runtime_authority"`
    ObservedProcessStart string `json:"observed_process_start,omitempty"`
    WindowStart string `json:"window_start,omitempty"`
    WindowEnd string `json:"window_end,omitempty"`
    RequestCount int `json:"request_count"`
    JetBrainsRequestCount int `json:"jetbrains_request_count"`
    AIGWRequestCount int `json:"aigw_request_count"`
    OtherRequestCount int `json:"other_request_count"`
    HostHashes []string `json:"host_hashes"`
    HostAuthentication string `json:"host_authentication"`
    BillingEvidence string `json:"billing_evidence"`
    EvidenceSource string `json:"evidence_source"`
    ReadOnly bool `json:"read_only"`
}

type AirOptions struct {
    LogDir string
    AIGWEndpoint string
    ConfigurationState string
    Now time.Time
}

func InspectAirRuntime(options AirOptions) (AirRuntimeAttestation, error)
func AirLogDirFor(goos string, env map[string]string) (string, error)
```

- [ ] **Step 1: Write failing parser/path tests**

Use admitted lines with an anchored timestamp/PID prefix, exact logger
`CodexOpenAiApiRouterServer`, and exact message
`Forwarding CallTraceId(id=<bounded>)/POST:/responses to <absolute-URL>`.
Cover AIGW-only, JetBrains-only, mixed, other-only, newest Air generation,
same-PID rotations, stale/future timestamps, malicious suffixes, malformed URL,
HTTP JetBrains, overlong lines, total-byte cap, and ignored `Headers:` and
`Request body:` lines.

- [ ] **Step 2: Run the failing tests**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/attestation ./internal/platform -count=1
```

Expected: FAIL because the package and path function do not exist.

- [ ] **Step 3: Implement strict parsing and caps**

Use a 24-hour freshness bound, 64 KiB line cap, 16 MiB total scan cap, and
128-byte trace-ID cap. Select the latest admitted PID from `air.log`; use
`air1.log` through `air9.log` only for the same generation. Parse URLs with
`net/url`, admit JetBrains only for HTTPS exact `jetbrains.ai` or suffix
`.jetbrains.ai`, and compare full normalized AIGW route identity internally.

- [ ] **Step 4: Implement aggregation and redaction tests**

Set runtime authority exactly to `jetbrains-ai`, `aigw`, `mixed`, or `unknown`.
Set `host-mirror-runtime-attested` only for a JetBrains-only fresh host mirror.
Hash normalized route authority with domain separator
`aigw-air-route-host-v1\x00`; sort and deduplicate. Assert marshalled reports
and errors omit every raw fixture value and path.

- [ ] **Step 5: Verify and commit**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/attestation ./internal/platform -count=1
git add internal/attestation internal/platform/paths.go internal/platform/paths_test.go
git commit -m "feat: parse secret-free Air route evidence"
```

---

### Task 4: Add read-only `aigw route attest air`

**Files:**
- Create: `internal/cli/route_attest.go`
- Test: `internal/cli/route_attest_test.go`
- Modify: `internal/cli/advanced.go`
- Modify: `internal/cli/app.go`
- Test: `internal/cli/mutation_test.go`

**Interfaces:**

```go
// App fields
DataDir string
AirLogDir string
Now func() time.Time

func newRouteAttestCommand(app *App) *cobra.Command
func runAirAttest(app *App, jsonMode bool) error
```

- [ ] **Step 1: Write failing CLI tests**

Add `TestAirRouteAttestReportsRuntimeBoundToHostMirror`,
`TestAirRouteAttestUnknownEvidenceDoesNotClaimRuntime`,
`TestAirRouteAttestIsReadOnlyAndPathFree`, and
`TestAirRouteAttestRejectsNonAirSurface`. Assert the positive JSON contains
`external-host-mirror`, `host-mirror-runtime-attested`, and `jetbrains-ai`.

- [ ] **Step 2: Run the failing tests**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/cli -run 'TestAirRouteAttest|TestRouteAttest' -count=1
```

Expected: FAIL because `attest` is not registered.

- [ ] **Step 3: Register and implement the command**

Accept only `aigw route attest air --json`. Resolve Air and standalone, call the
Air-aware classifier, resolve the public configured Codex endpoint without
reading `app.Secrets`, and call `InspectAirRuntime`. `NewDefault` populates data
and log dirs with platform helpers and `Now` with `time.Now`.

- [ ] **Step 4: Prove lock-free behavior**

Do not add `attest` to a mutating branch in `mutationCommand`. Assert execution
does not call `Config.Lock`, create recovery directories, or change any file.

- [ ] **Step 5: Verify and commit**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/cli -run 'TestAirRouteAttest|TestRouteAttest|TestMutation' -count=1
git add internal/cli/route_attest.go internal/cli/route_attest_test.go internal/cli/advanced.go internal/cli/app.go internal/cli/mutation_test.go
git commit -m "feat: attest Air runtime routing from bounded logs"
```

---

### Task 5: Add the recovery ledger and quarantine transaction

**Files:**
- Create: `internal/recovery/ledger.go`
- Test: `internal/recovery/ledger_test.go`
- Create: `internal/recovery/air.go`
- Test: `internal/recovery/air_test.go`

**Interfaces:**

```go
type LedgerState string

const (
    StatePrepared LedgerState = "prepared"
    StateAwaitingHostRoundtrip LedgerState = "awaiting-host-roundtrip"
    StateQuarantined LedgerState = "quarantined"
    StateSettled LedgerState = "settled"
)

type AirLedger struct {
    SchemaVersion int `json:"schema_version"`
    SurfaceID string `json:"surface_id"`
    RecoveryGeneration uint64 `json:"recovery_generation"`
    CaseID string `json:"case_id"`
    State LedgerState `json:"state"`
    CreatedAt time.Time `json:"created_at"`
    RecoveredAt time.Time `json:"recovered_at,omitempty"`
    SettledAt time.Time `json:"settled_at,omitempty"`
    ProjectionFingerprintSHA256 string `json:"projection_fingerprint_sha256"`
    ConfigPreimageSHA256 string `json:"config_preimage_sha256"`
    ConfigPreimageMode uint32 `json:"config_preimage_mode"`
    CleanedPostimageSHA256 string `json:"cleaned_postimage_sha256"`
    ObservedRoundtripSHA256 string `json:"observed_roundtrip_sha256,omitempty"`
    QuarantineSHA256 string `json:"quarantine_sha256"`
}

type AirStore struct { Root string; Now func() time.Time }
type SettlementKind string

const (
    SettlementUnchanged SettlementKind = "unchanged-cleaned-postimage"
    SettlementExternalClean SettlementKind = "external-clean-roundtrip"
    SettlementHostMirror SettlementKind = "external-host-mirror-roundtrip"
    SettlementUnexpected SettlementKind = "unexpected-residue"
)

func (s AirStore) PlanRecover(configPath, projectionFingerprint string, cleaned []byte) (RecoverPlan, error)
func (s AirStore) ApplyRecover(plan RecoverPlan) (AirLedger, error)
func (s AirStore) Load() (AirLedger, error)
func (s AirStore) PlanSettle(caseID, configPath string, kind SettlementKind) (AirLedger, error)
func (s AirStore) ApplySettle(caseID, configPath string, kind SettlementKind) (AirLedger, error)
```

`RecoverPlan` exposes only case ID, generation, action, surface, and
configuration state in JSON; captured paths, bytes, and snapshots remain
private.

- [ ] **Step 1: Write failing ledger/case tests**

Generation starts at one and case ID is
`air-%06d-<first-12-preimage-sha256>`. Reject unknown JSON fields, bad schema,
bad digest, mismatched ID, illegal transition, and content/path-like keys.

- [ ] **Step 2: Write failing transaction tests**

Cover plan-without-directory-creation, successful quarantine/cleaning,
permissions, deterministic re-plan, different active case rejection,
`prepared` resume before/after config write, concurrent preimage change, and
injected failures at each artifact boundary. Assert byte-exact reverse rollback.

- [ ] **Step 3: Run the failing tests**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/recovery -count=1
```

Expected: FAIL because the package does not exist.

- [ ] **Step 4: Implement ledger validation and recovery order**

Use `DisallowUnknownFields`. Store ledger at `<root>/ledger.json` and raw
preimage at `<root>/quarantine/<case-id>/config.toml`. Capture all snapshots,
then commit quarantine, `prepared` ledger, cleaned config, and
`awaiting-host-roundtrip` ledger. Use existing guarded transaction helpers and
reverse compensation. Resume only the identical deterministic case.

- [ ] **Step 5: Implement settlement tests and behavior**

Unchanged cleaned postimage rejects without a write. External-clean or positive
host-mirror roundtrip settles. Unexpected residue records `quarantined` without
changing Air. Successful settle records observed hash, removes only matching
quarantine, retains hashes-only ledger, and is idempotent. Inject ledger and
quarantine failures and prove compensation.

- [ ] **Step 6: Verify and commit**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/recovery -count=1
git add internal/recovery
git commit -m "feat: ledger Air orphan recovery cases"
```

---

### Task 6: Add `recover-orphan` and `settle`

**Files:**
- Create: `internal/cli/route_orphan.go`
- Test: `internal/cli/route_orphan_test.go`
- Modify: `internal/cli/advanced.go`
- Modify: `internal/cli/app.go`
- Test: `internal/cli/mutation_test.go`
- Modify: `internal/cli/route_doctor.go`
- Test: `internal/cli/route_doctor_test.go`

**Interfaces:**

```go
func newRouteRecoverOrphanCommand(app *App) *cobra.Command
func newRouteSettleCommand(app *App) *cobra.Command
func runAirRecoverOrphan(app *App, dryRun, jsonMode bool, caseID string, confirmHostIdle, ackUnsetExternalSelection bool) error
func runAirSettle(app *App, dryRun, jsonMode bool, caseID string) error
```

- [ ] **Step 1: Write failing recover tests**

Add tests for stable no-write preview, wrong/missing case ID, missing idle/ack,
no `--force`, mirror/partial/sidecar/listed-target rejection, successful unset
cleanup, journal resume, path-free JSON, and no Runner/Secrets call.

- [ ] **Step 2: Run the failing tests**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/cli -run 'TestAirRecoverOrphan|TestAirSettle' -count=1
```

Expected: FAIL because neither command exists.

- [ ] **Step 3: Implement `recover-orphan`**

Register:

```text
aigw route recover-orphan air --dry-run --json
aigw route recover-orphan air --case-id <id> --confirm-host-idle --ack-unset-external-selection
```

Reject configured Air target membership. Compose `PlanAirOrphanRemoval` with
`AirStore.PlanRecover`. Dry-run renders bounded plan fields. Apply checks all
bindings and calls `ApplyRecover` without credentials, sidecar, or client work.

- [ ] **Step 4: Write and implement settle tests**

Register:

```text
aigw route settle air --case-id <id> --dry-run --json
aigw route settle air --case-id <id>
```

Cover unchanged cleaned state, external-clean, exact current host mirror,
reappeared same fingerprint without reference proof, partial/different residue,
wrong case, changed quarantine, and already settled. Settle never writes Air.

- [ ] **Step 5: Project ledger state into doctor**

Keep configuration and recovery state separate. Report persistent
`awaiting-host-roundtrip`, `quarantined`, or `settled`, and derive
`reappeared-after-recovery` when the same projection returns while awaiting.

- [ ] **Step 6: Update mutation classification**

`recover-orphan` and `settle` mutate only without `--dry-run`; `attest` remains
read-only. Add explicit lock-seam tests for all five forms.

- [ ] **Step 7: Verify and commit**

```bash
GOTOOLCHAIN=go1.25.12 go test ./internal/cli -run 'TestAirRecoverOrphan|TestAirSettle|TestRouteDoctor|TestMutation' -count=1
git add internal/cli/route_orphan.go internal/cli/route_orphan_test.go internal/cli/advanced.go internal/cli/app.go internal/cli/mutation_test.go internal/cli/route_doctor.go internal/cli/route_doctor_test.go
git commit -m "feat: recover and settle exact Air orphans"
```

---

### Task 7: Promote the decision and prepare RC.67 source

**Files:**
- Create: `docs/decisions/0004-air-host-mirror-and-orphan-recovery.md`
- Modify: `README.md`
- Modify: `docs/architecture/authority-and-projection-boundary.md`
- Modify: `docs/security.md`
- Modify: `docs/governance/terminal-experience-contract.md`
- Modify: `CHANGELOG.md`
- Delete after promotion: `docs/superpowers/specs/2026-07-19-air-host-mirror-recovery-design.md`
- Delete after closeout: `docs/superpowers/plans/2026-07-19-air-host-mirror-recovery.md`

- [ ] **Step 1: Write ADR-0004 and canonical documentation**

Record exact config states, ephemeral runtime state, log boundary, deterministic
case ID, acknowledgements, quarantine, state machine, unset baseline, and the
separation from ADR-0003 and runtime authentication/billing/terminal proof.

- [ ] **Step 2: Add the exact Changelog heading**

Keep `## [Unreleased]` first, then add:

```markdown
## [0.1.0-rc.67] - 2026-07-19

### Fixed

- Distinguish a verified JetBrains Air host mirror from a true exact AIGW
  orphan, and recover only the latter through a case-bound quarantine and
  settlement workflow.
- Add a secret-free, read-only Air route attestation from bounded forwarding
  evidence without exposing routes, credentials, prompts, or sessions.
```

Do not edit `internal/cli/root.go`, toolchain carriers, forge sources, or signer
anchors.

- [ ] **Step 3: Run documentation gates and commit**

```bash
python3 scripts/check-markdown-presentation.py
python3 scripts/check-text-layout.py
sh scripts/test-text-layout.sh
sh scripts/check-english-text.sh
sh scripts/check-governance.sh
sh scripts/test-changelog.sh
git add README.md CHANGELOG.md docs
git commit -m "docs: record Air host-mirror recovery decision"
```

---

### Task 8: Run source gates and release RC.67

**Files:**
- Verify only: repository and release artifacts.

- [ ] **Step 1: Run the complete local source bundle**

```bash
export GOTOOLCHAIN=go1.25.12
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd internal tools)"
for f in scripts/*.sh; do sh -n "$f"; done
sh scripts/check-release-toolchain.sh
sh scripts/check-package-runner.sh
sh scripts/check-package-safety.sh
sh scripts/check-product-surface.sh
sh scripts/check-retired-residue.sh
sh scripts/check-english-text.sh
sh scripts/check-governance.sh
python3 scripts/check-markdown-presentation.py
python3 scripts/check-text-layout.py
sh scripts/test-text-layout.sh
sh scripts/test-changelog.sh
sh scripts/test-github-provider-projection.sh
sh scripts/test-github-actions-contract.sh
sh scripts/test-github-release-workflow.sh
sh scripts/test-pipeline-gates.sh
pwsh -NoProfile -File ./scripts/test-installers.ps1 -Installer ./scripts/install.ps1
```

Expected: every command PASS. Stop before tagging on any failure.

- [ ] **Step 2: Build the formal matrices on the dedicated macOS arm64 runner**

```bash
export GOTOOLCHAIN=go1.25.12
TAG=v0.1.0-rc.67
VERSION=${TAG#v}
SOURCE_DATE_EPOCH=$(sh scripts/release-source-date-epoch.sh "$VERSION")
test "$SOURCE_DATE_EPOCH" = 1784419200
export SOURCE_DATE_EPOCH
sh scripts/test-release-reproducibility.sh "$VERSION"
AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh "$VERSION" dist
sh scripts/test-release-package-layout.sh dist "$VERSION"
sh scripts/check-release-artifacts.sh dist "$VERSION"
```

Expected: two byte-identical 15-artifact matrices, binary version
`0.1.0-rc.67`, and MSI ProductVersion `0.1.194`.

- [ ] **Step 3: Enforce provider-native provenance and publication order**

Stop if an RC.67 GitLab package or Release already exists; the existing reuse
path does not byte-compare local and remote assets. Create separate SSH-signed
annotated `v0.1.0-rc.67` tag objects with the GitLab and GitHub identities and
their own tracked anchors. Run `sh scripts/project-github-forge.sh`; never copy
or overwrite tags between providers.

- [ ] **Step 4: Accept only dual-provider evidence**

Publish GitLab first. After its release succeeds, push the GitHub tag and
require both GitHub Verify and Release workflows. Download both matrices and
run the `scripts/compare-release-artifacts.sh` contract. Record source,
publication, native, and deferred evidence separately. Do not claim GA signing,
notarization, host-enforced GitHub immutability, Air authentication, billing, or
a visible reply.

---

## Self-review

- [ ] Every design state has a named test and implementation task.
- [ ] Every write has a failure-injection, preimage, rollback, and retry test.
- [ ] Every read-only command has no-lock, no-directory, no-secret, and
  path-free tests.
- [ ] ADR-0003 remains a separate regression-covered recovery path.
- [ ] No task changes session storage, Air lifecycle, credentials,
  `internal/cli/root.go`, toolchain pins, or provider trust manifests.
- [ ] RC.67 date, epoch, binary/MSI versions, artifact count, and dual-forge
  constraints are exact.
