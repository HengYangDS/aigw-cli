## Context

GitLab runs AIGW's jobs on three Parallels guests of the operator's Mac. The
Linux jobs run the Mise image that the CUE authority pins by digest, and the
Windows jobs verify a runner-owned Mise in `C:\Program Files\mise-aigw-cli\bin`
by digest and version (`windows-mise-directory`, 2026-11-03). The four macOS
jobs call whatever `mise` the runner account's `PATH` finds: the host's shared
Homebrew Mise, which the CI infrastructure owner updates for six projects. No
job reads its version. GitHub's hosted jobs install the declared version with
mise-action.

## Goals / Non-Goals

**Goals:**

- Every AIGW Forge job runs Mise 2026.11.7, the newest stable release on
  2026-11-13, and fails before the locked toolchain installs if it would run
  another one.
- Moving AIGW's Mise leaves the other projects' shared Mise untouched.

**Non-Goals:**

- Upgrading the macOS host's shared Homebrew Mise, which other projects use.
- Raising the repository's Mise floor.
- Installing Mise per job.

## Decisions

### A runner-owned Mise on macOS, as on Windows

AIGW's macOS jobs verify `/opt/mise-aigw-cli/bin/mise` against the CUE digest
and version, then put its directory first on `PATH`. The directory is
root-owned and read-only for the runner accounts, like `Program Files` on
Windows. Its release replaces the executable in a window without AIGW jobs, as
the Windows one does.

Rejected: upgrading the shared Homebrew Mise. It belongs to six projects and
to the host's owner, so a declaration of one project must not move it, and a
Homebrew upgrade cannot pin an exact release by digest. Also rejected: an
installer step that downloads and verifies Mise in every job. It adds
installation logic to the pipeline and a GitHub download to every job, where
a verified file on the host needs one check.

### Only the executable

The macOS release archive holds `bin/mise` and `bin/mise.d`. `mise.d` is the
compiler's dependency list from the release build, not a runtime file, so the
host directory holds only `mise`. Windows keeps both `mise.exe` and
`mise-shim.exe`, which its jobs verify.

### The floor stays

No repository test asserts a diagnostic that only a newer Mise prints, and the
lock that 2026.11.7 writes installs under 2026.10.25, as the 2026-11-13 dev
pipelines on 2026.10.25 runners showed. A higher floor would stop developers on
an older Mise without protecting any job, which the pins already do.

### Verification of the new release

The pinned digests come from the release: `SHASUMS256.txt` names the macOS
executable (`0f7cb85b…`) and the Windows archive, whose `mise.exe` and
`mise-shim.exe` digests (`e4f2b674…`, `b5a1c01a…`) were read from the extracted
archive. The Linux image index digest (`0e4c7634…`) has linux/arm64 and
linux/amd64 manifests. A fresh locked install of the repository under 2026.11.7,
with new data, cache, state and configuration directories, installed all 21
tools and left `mise.lock` unchanged.

## Risks / Trade-offs

- A job on a host without the new executable fails at its first step until the
  host is updated. The window installs both executables before the proposal is
  published, and rollback restores the previous files kept on each host.
- The macOS host now holds two Mise releases, about 150 MB.
