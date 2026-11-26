## Context

GitLab's Linux jobs run the Mise image that the CUE authority pins by index
digest, and GitHub's jobs install `miseVersion`, derived from that image's tag,
with mise-action. AIGW's four GitLab jobs on the macOS ARM64 host verify the
digest and version of AIGW's own Mise at `/opt/mise-aigw-cli/bin/mise` before
any locked tool runs (`mise-2026-11-7`). Since `drop-windows-and-intel-macos`
that host is the only runner that owns AIGW's Mise. Its path names no release,
so the previous move replaced the file in a window with the runners paused.

## Goals / Non-Goals

**Goals:**

- Every AIGW Forge job runs Mise 2026.11.14 and fails before the locked
  toolchain installs if it would run another release.
- A later move of the declaration needs no window without jobs on the macOS
  host and changes no file that a running job verifies.

**Non-Goals:**

- Installing or removing Mise on the host; the owner of the CI hosts does that
  (Migration Plan).
- Raising the repository's Mise floor.
- Moving the host's shared Homebrew Mise, which other projects use.

## Decisions

### The directory names its release

`miseMacosArm64Directory` is `/opt/mise-aigw-cli/\(miseVersion)/bin`, so the
one version literal in `miseImage` also selects the macOS directory. Each
pipeline runs the preflight text of its own commit's `.gitlab-ci.yml`: a job of
the earlier declaration verifies the earlier directory, and a job of this
declaration verifies the new one. Both releases can stay on the host, and
installing one or removing the other touches no file that a job of the other
declaration reads. Both GitLab macOS projection tests fail when the declared
directory does not name the declared release; the check compares the two
declarations with each other and repeats neither.

Rejected: keeping `/opt/mise-aigw-cli/bin` and replacing the file in a window
without jobs, as `mise-2026-11-7` did. From the replacement on, every pipeline
of an earlier head fails its digest check, and the window needs the runners
paused. Also rejected: a `current` link that the host switches at landing. It
changes the executable under running jobs and adds a host step at the moment
of landing.

### The floor stays at 2026.10.25

`min_version` is the oldest Mise that may read the repository's configuration,
not the release that runs (`docs/governance/dependency-maintenance.md`). Mise
treats a plain `min_version` as a hard minimum: an older release stops with an
error (Mise's configuration reference at v2026.11.14). No AIGW CI job installs
ETHOS's own Mise (every job runs the declared release through the image, the
runner-owned Mise or mise-action), so nothing in CI bounds the floor from below.
The supply-chain requirement raises the floor with the pin only when a
repository test asserts a diagnostic of the newer release, and no test asserts
a Mise diagnostic that 2026.11.8 to 2026.11.14 introduced; the floor therefore
stays.

### mise-action v5.3.0

v5.2.10 fixes a download on Windows when an MSYS `curl.exe` comes first on
`PATH`, and v5.3.0 adds an optional `mise_env` input that AIGW leaves unset
(both release notes, read 2026-11-26). The inputs AIGW passes keep their
meaning. The pin is commit `924e7c586e33d7d4bae9c14e26c5d22e2beb6e78`, which
the lightweight tag v5.3.0 names, with a verified signature; the release was
published on 2026-11-21, older than the release age.

### Verification of the new release

- GitHub's `releases/latest` for jdx/mise names v2026.11.14, published
  2026-11-24T11:00:43Z, not a prerelease; its tag names commit `383f8589`.
- The macOS digest `ad39dddb…` is the `SHASUMS256.txt` line for
  `mise-v2026.11.14-macos-arm64`, the bare executable, the asset kind whose
  line gave `mise-2026-11-7` its digest. GitHub's own digest of that asset is
  the same.
- The Linux index digest `a7df8bea…` of `jdxcode/mise:2026.11.14-debian` came
  from Docker Hub's registry API with an anonymous pull token; the downloaded
  index hashes to the same value and lists linux/amd64 and linux/arm64
  manifests. The arm64 image labels version 2026.11.14 and revision
  `383f8589`. The same reading of `2026.11.7-debian` returns `0e4c7634…`, the
  earlier pin.
- Under the host's Mise 2026.11.14, a locked install of the lane with new
  data, cache, state and configuration directories installed all 21 locked
  tools without a warning and left `.config/mise/mise.lock` byte for byte
  unchanged (2026-11-26).
- The release notes of 2026.11.8 to 2026.11.14 name one breaking change, to
  Temurin version names, which AIGW does not lock. `mise lock` may now write an
  `unsupported_platforms` list; a locked install writes no lock.
- On the GitLab macOS host, `/opt/mise-aigw-cli/2026.11.14/bin/mise` is
  root:wheel with mode 0755, 146,963,104 bytes, digest `ad39dddb…` and
  TeamIdentifier 4993Y37DX6; under both AIGW runner accounts, `ci-aigw-review`
  and `ci-aigw-protected`, `--version` prints
  `2026.11.14 macos-arm64 (2026-11-24)`, and neither account can write it;
  `/opt/mise-aigw-cli/bin/mise` is unchanged. This is the relayed reading of
  `ETHOS 仓库纳管` at 2026-11-26 06:35, not one of this lane.

## Risks / Trade-offs

- [A job runs before the host holds the new release] → It stops at its first
  step with "Runner-owned Mise is missing." before any locked tool runs. The
  Migration Plan installs the release before the proposal's first pipeline.
- [The earlier directory goes while a job of an earlier head still runs] → That
  job fails closed at the same step. Removal waits until no job consumes the
  earlier declaration.
- The host holds both releases, 146,963,104 and 142,892,304 bytes, until the
  removal.

## Migration Plan

The cutover, in order:

1. `ETHOS 仓库纳管`, the owner of the CI hosts, installs the verified
   executable as `/opt/mise-aigw-cli/2026.11.14/bin/mise`, root-owned and not
   writable by the runner accounts, beside `/opt/mise-aigw-cli/bin/mise`, and
   reads it back under both AIGW runner accounts. No AIGW job reads the new
   path yet, so this needs no window.
2. The proposal's GitLab pipeline runs. Its only macOS job is
   `native-darwin-review`, under the review account, and it prints the release
   it verified. The protected account's jobs first run after landing:
   `native-darwin` in the dev push pipeline, the other two in the main push
   and tag pipelines.
3. This Change lands.
4. Once dev and main carry this Change and no open proposal runs an earlier
   declaration, `ETHOS 仓库纳管` removes `/opt/mise-aigw-cli/bin`; the check,
   the refs that still verify the earlier path and the review date are in the
   tasks' Workflow follow-up.

Before step 4 a rollback is a revert of the declaration alone, since the
earlier executable is still in place. Each later release follows the same
order with its own directory.
