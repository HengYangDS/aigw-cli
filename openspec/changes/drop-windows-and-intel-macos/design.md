## Context

The proposal gives the owner's words and the scope. A read-only inventory of `dev` at `d6d76d121` (identical under
`internal/`, `tools/`, `.config/` and `openspec/specs/` to the lane's base `b992efcbc`) found about 200 files that
carry Windows or Intel macOS; it is kept at `build/evidence/windows-removal-inventory-20261125.md` in the main
checkout. Measured on GitLab from 11-24 20:00 to 11-25 (eight aigw pipelines, read from the GitLab API by the
session 「迭代加速与性能」): `native-windows` 23.8 to 39.1 minutes, `native-darwin` 19.7 to 28.9, `native-linux`
7.8 to 12.0, `quality` 5.5 to 9.3.

## Goals / Non-Goals

**Goals:** one supported set (macOS ARM64, Linux ARM64, Linux AMD64) declared once per concern; no code, test,
pipeline, lock row, document or requirement that exists only for Windows or Intel macOS; the two forges' overlap
rendered from one declaration.

**Non-Goals:** a new platform, a new check or a new required check; the proxy repository; the CI guests and runner
registration; the release that ships the Change; unifying declarations that this Change does not otherwise touch.

## Decisions

**D1. The supported set.** Releases publish `darwin-arm64`, `linux-arm64`, `linux-amd64`. The release matrix is
declared in `.config/release/goreleaser/.goreleaser.yaml` (what GoReleaser builds) and in
`internal/program/artifact/identity.go` (what the program and its acceptance accept); both drop to the three
targets in the same commit, and the existing tests that enumerate targets follow. Alternative rejected: keeping
`darwin-amd64` as a cross-compiled artifact, because a published artifact needs native execution of its lifecycle
(`AGENTS.md`, engineering quality) and no runner remains for it.

**D2. Native evidence per operating system.** The CUE graph keeps one native job per operating system: macOS and
Linux. GitHub runs them on `macos-latest` and `ubuntu-24.04`; GitLab on its ARM64 macOS guest (tags `macos-latest`
and `review` for proposal and merge-request pipelines, `protected` for `dev`, `main` and tags) and its ARM64 Linux
guest. The macOS job is one declaration with one tag pair per pipeline kind, rendered to both forges. GitHub gains
no ARM64 Linux verify job: native source evidence is per operating system, and the published `linux-arm64`
lifecycle is verified through the release workflow's `ubuntu-24.04-arm` choice, as today. GitHub's required checks
keep their names; only `Native Windows acceptance` leaves.

**D3. One shell.** GitHub jobs ran `pwsh` on every runner so that one script served Windows. They run the runner's
default `bash` now; GitLab's PowerShell job templates leave with the Windows jobs.

**D4. Code.** Every `_windows.go` file and every `//go:build windows` file leaves, with the Windows-only module
requirement. A `!windows` or `unix` constraint leaves its file; where a POSIX file was the twin of a Windows file,
it takes the plain name. A `runtime.GOOS == "windows"` branch, a Windows test case and a fixture for another
operating system that AIGW does not support leave. A seam whose only second implementation was Windows collapses
into its one implementation (the inventory names `internal/secret/native` dispatch, `terminal.ColorEnabled`'s
operating-system parameters, `process.Plan.DebugProcess`, `tools/release/performance.ObserveCurrent`, and
`tools/ci`'s `supplyInUse` and `qualifyProtectedResource`); each is confirmed in the code before it collapses.
The darwin Keychain, the Linux Secret Service and the file backend stay unchanged.

**D5. Bounds derived from the Windows job.** `tools/ci`'s selection-matrix bound and `matrixJobMinutes` were set
from the Windows run. Each is re-derived from the slowest remaining native job, citing its measured run, and tests
pin the derivation, not the number.

**D6. Toolchain.** The Mise lock keeps the rows of macOS ARM64, Linux ARM64 and Linux AMD64. The rows are removed
by Mise's own lock command restricted to those platforms; the diff of the lock removes rows and changes no
version, checksum or URL of a kept row. Asset patterns and templates written only for Windows leave
`.config/mise/config.toml`.

**D7. Records.** Documents state the supported set once, in the release policy, and link to it. A decision record is
a dated record: where one names Windows as current behavior, it gets a dated amendment line naming this Change, and
its history stays. `CHANGELOG.md` records the breaking removal under the next release; earlier entries stay.

**D8. Specifications.** A requirement that exists only for Windows is REMOVED with a one-sentence reason. A
requirement that names Windows beside macOS and Linux is MODIFIED; a MODIFIED block keeps every scenario name
(OpenSpec), so a scenario that exists only for Windows leaves through REMOVED plus ADDED under a new requirement
name. Each requirement stays within 500 characters and each delta within the 500-line size gate.

**D9. Order with the forges.** Before the first push of `dev` to GitHub, `Native Windows acceptance` leaves the
required checks of `dev` and `main` (owner, 14:52), and both are read back. Before the first GitLab push, ETHOS
仓库纳管 confirms that aigw-cli's macOS runners carry the new tags (asked 16:4x); the old tags are dropped by it
after `main` is published. After the Change is on `dev` and `main` of both forges, ETHOS 仓库纳管 is told, for the
Windows guest step of its decision D54.

## Risks / Trade-offs

- A user on Windows or Intel macOS has no supported program after the next release → the release notes say so; the
  last release with those artifacts stays published.
- `macos-latest` moves when GitHub moves it → the first job's log gives the macOS version and architecture, read
  before acceptance; the artifact's deployment target is declared separately and does not follow the runner.
- A collapsed seam loses a test double that tests used → a test that needed the double is rewritten against the one
  implementation or removed if it only exercised Windows.
- The GitLab macOS tags are not ready when the proposal is pushed → no push until ETHOS 仓库纳管 answers.

## Migration Plan

None in product code. The owner's installed program is ARM macOS and upgrades through the cask as usual.
