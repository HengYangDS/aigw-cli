## Why

On 2026-11-25 the owner withdrew Windows support from every repository that ETHOS governs, from CI and from
every other aspect (13:45, relayed by ETHOS 仓库纳管). GitLab runs only ARM macOS and, where needed, ARM Linux;
GitHub runs only ARM macOS and, where needed, ARM and x86 Linux. Both forges name their macOS runner
`macos-latest` (13:54, relayed; restated first-hand at 14:52). At 14:52 the owner confirmed two readings: Windows
leaves AIGW entirely, and Intel macOS goes too, so `darwin-amd64` leaves the release matrix. The same day the owner
restated that each project holds with no remote, one remote or two, local first, each remote independent, and that
where two remotes overlap their declaration is one mirrored declaration (13:47, relayed).

AIGW today builds, tests, accepts and documents three operating systems and six release targets. About 200 files
carry Windows or Intel macOS: 37 Windows-only Go files, a Credential Manager and DPAPI token backend, Job Object
process capture, Windows path namespaces, two GitLab Windows jobs, a GitHub Windows job, release archives in zip
form, the Mise lock rows of three platforms, and twelve accepted specifications. GitLab's aigw pipeline waits 24 to
39 minutes for its Windows job, the longest of its jobs.

## What Changes

- **BREAKING** AIGW supports macOS on ARM64 and Linux on ARM64 and AMD64 only. Releases publish
  `darwin-arm64`, `linux-arm64` and `linux-amd64`; there is no Windows archive, no `.exe`, no zip and no
  `darwin-amd64` archive. The Homebrew cask is ARM64 only.
- **BREAKING** The Windows token backends (Credential Manager and the DPAPI file) and every Windows code path,
  build constraint, test branch and fixture are removed. A seam left with one implementation collapses into it.
- CI: the one CUE graph declares a native job for macOS and for Linux. GitHub runs them on `macos-latest` and
  `ubuntu-24.04`; GitLab runs them on its ARM64 macOS guest (`macos-latest` with `review` or `protected`) and its
  ARM64 Linux guest. The macOS job is one declaration rendered to both forges. GitHub's release verification keeps
  `ubuntu-24.04-arm` as a runner choice for the published `linux-arm64` lifecycle.
- The Mise lock keeps only the platforms that CI and releases use: macOS ARM64, Linux ARM64, Linux AMD64.
- Documents, `AGENTS.md`, the decision records that name Windows, and the accepted specifications say the same.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `release-distribution`, `program-lifecycle`: three release targets; Windows path derivation and the Windows
  lifecycle leave.
- `native-credential-store`, `secret-storage`, `credential-continuity`: no Windows backend or reader.
- `product-quality`, `quantitative-quality`, `ci-diagnostics`, `repository-organization`, `supply-chain`: native
  evidence on macOS and Linux; no Windows job, runner, supply or fixture.
- `cli-readiness`, `model-selection`: no Windows path cases.

## Impact

Product and tool code under `internal/` and `tools/`, `go.mod` (the Windows-only `wincred` requirement leaves),
`.config/ci/pipeline.cue` and the three generated pipeline files, `.config/release/goreleaser/.goreleaser.yaml`,
`.config/mise/`, the documents and specifications listed in the design. A user on Windows or Intel macOS loses the
supported program; the owner runs ARM macOS. GitHub's required checks on `dev` and `main` drop
`Native Windows acceptance` when this Change publishes `dev` (owner's approval of 14:52); the other required check
names stay.

Out of scope: the `openai-responses-proxy` repository (its own Change); the CI guests themselves (retiring the
Windows guest and the runner tags belong to ETHOS 仓库纳管 with the owner's word); the release that ships this
Change; any new platform or check.
