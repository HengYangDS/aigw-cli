# Design

## Context

`construction.NativeAcceptanceClients` is the release owner's registry of
client journeys per platform. `requireNativeClients` refuses acceptance until
every declared client has an absolute executable in `AIGW_ACCEPTANCE_<CLIENT>`
and Hermes has its installation home in `AIGW_ACCEPTANCE_HERMES_HOME`.
`ci native-clients supply` installed only the Windows npm packages, and the CUE
model added its steps and the `node` tool only to the Windows job.

Verified official layouts for the pinned releases, Codex `0.163.0` and Claude
Code `2.1.302`:

| Host          | Codex executable under `node_modules/@openai/`                | Claude executable under `node_modules/@anthropic-ai/` |
| ------------- | ------------------------------------------------------------- | ----------------------------------------------------- |
| linux/amd64   | `codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/codex`  | `claude-code-linux-x64/claude` (glibc)                |
| darwin/arm64  | `codex-darwin-arm64/vendor/aarch64-apple-darwin/bin/codex`    | `claude-code-darwin-arm64/claude`                     |
| windows/amd64 | `codex-win32-x64/vendor/x86_64-pc-windows-msvc/bin/codex.exe` | `claude-code-win32-x64/claude.exe`                    |

Hermes publishes no release assets. Its official installation on Linux and
macOS is the release's own `scripts/install.sh`. That installer clones the
release, acquires uv and Python, and publishes `~/.local/bin/hermes`. Stable
`v0.22.5` is commit `bf0203a794a07aac3ff764972ab9087baa4e4f4e`. Its installer
SHA-256 is `99cb3691d55019c6973a7acb775a915648666cc24e483ca827a1cdbb3531a674`.

## Goals / Non-Goals

**Goals:** each GitHub native job can supply the complete official clients its
host declares. The supply exports the explicit inputs that acceptance already
requires and leaves no supply state behind after any outcome.

**Non-Goals:**

- changing the client registry, a journey, or acceptance's refusal of missing
  inputs;
- Windows Hermes, which acceptance still refuses until launcher publication
  can be isolated;
- real-client supply in GitLab's Linux and macOS jobs.

## Decisions

### The release owner's registry selects the clients

The supply maps `runtime.GOOS` through `NativeAcceptanceClients` and supplies
exactly that list. A declared client without an official supply, or a host
without a verified layout, fails before any effect. Hermes pins are required
when the host declares Hermes and refused when it does not. A separate list of
clients in CI was rejected, because the two lists could drift.

### The pipeline model pins releases; the supply owns layouts

CUE passes `--codex`, `--claude`, and on Linux and macOS
`--hermes-commit` and `--hermes-installer-sha256`. The supply derives the npm
package names and executable paths from the host. Passing package
specifications per platform from CUE was rejected, because CUE would then have
to keep each host's package names consistent with paths that only the supply
knows.

### Hermes runs its own installer, verified and confined

The supply downloads the installer at the pinned commit and compares its
SHA-256 before running it. It then runs the installer with `--commit`, a
private `--dir` and `--hermes-home`, `--non-interactive`, and the
browser and computer-use tool packs skipped. `HOME`, the XDG directories,
`TMPDIR` and the Git global configuration point inside the supply, so the
launcher and the shell profile edits also stay inside it.

The installer receives a declared environment, not the job's: those private
settings, `HERMES_RUNTIME_DIR` at the installer's own default tool store under
the private Hermes home, and only the host's `PATH`, proxy and TLS trust
settings. The installer reads `HERMES_RUNTIME_DIR`, `HERMES_REPO_URL` and
`HERMES_INSTALL_CHANNEL` from its environment, so an inherited job value could
otherwise move its uv and Python outside the supply or change its source.
Removing those names one by one was rejected, because any other inherited
setting that the installer or its tools read would remain open.

Afterwards the checkout must be the pinned commit, by mutual ancestry, with no
tracked change, and the launcher must exist. Rejected alternatives:

- Reimplementing the installer's bootstrap: it would be a parallel wrapper of
  vendor behavior.
- The Docker image: it is not a native host executable.
- This Mac's installation: it carries a local commit.

The installer is the vendor's own script for both POSIX hosts; the repository
adds no shell or PowerShell script.

### Cleanup is owned and proved

The supply records its directory in `GITHUB_ENV` before any installation. A
failed step removes the directory. Every `ci` command runs through the native
process owner, in its own process group on POSIX hosts and its own Job on
Windows. A SIGINT or SIGTERM to the supplier is caught: the owner kills the
running installer and every process it started, and the supply removes the
directory before it fails. A job cancellation signals only the supplier, so
forwarding the signal to the installer and waiting was rejected; an installer
that ignores it would keep the job alive.

Retirement removes only an absolute recorded path with the supply prefix, and
proves the path is gone. On Linux and macOS it first lists processes. A
process whose command line names a path inside the supply makes retirement
fail and keep the directory, so a client or Hermes gateway that outlived its
journey cannot be hidden.

### One input and one step definition

`native_clients` replaces `windows_clients`. Every native job forwards
`--clients` from it and declares the same supply and retirement steps. When
clients are requested, every job adds `node` to its tool closure.

The macOS job therefore runs the same supply as Linux. Its Claude, Codex and
Hermes journeys follow the registry, and no platform is silently skipped.

## Risks / Trade-offs

- **The Hermes installer needs the network and can print warnings.** The native
  diagnostic policy still applies to it. A vendor warning fails the supply
  rather than being hidden.
- **The process check reads command lines.** A process that names the supply
  only through its working directory is not detected. Only process IDs are
  reported.
- **Larger client jobs.** The client packages are about 400 MB, and Hermes adds
  its environment. A run that requests clients gets a longer job deadline.
- **Renamed input.** A dispatch that still uses `windows_clients` is rejected by
  GitHub as an unknown input; it is not ignored.

## Migration Plan

Dispatch GitHub Verify with `native_clients=true`, `baseline_tag` and
`candidate_tag` once per platform. The Linux run qualifies BLOCK `82f2a380`.

## Open Questions

- None.
