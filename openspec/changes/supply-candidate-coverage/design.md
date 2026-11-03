## Context

The CUE authority pins the native clients that the real-client journeys
install: the Codex and Claude Code npm versions, and the Hermes release commit
with the SHA-256 of its installer. The candidate calculation runs Renovate with
the repository policy, whose regex rules read the pins that no native Renovate
manager understands. The policy's rule for npm clients expects
`"<package>@<version>"` strings, the form that `aa55e2fc` replaced.

## Goals / Non-Goals

**Goals:** the candidate calculation reads every native client pin from its
publisher, a test keeps it that way, and the supply takes uv 0.13.7 and Hermes
v0.22.6.

**Non-Goals:** a release, a change to the release-age policy, a new candidate
tool, or the proxy's candidate calculation.

## Decisions

- One regex rule for each client keeps the CUE carrier of Codex and Claude
  Code unchanged. Each rule names its npm package, because the CUE key names
  the client, not the package. A single rule that maps keys to packages through
  a template was rejected as harder to read for no gain.
- The Hermes commit carries its release tag in a trailing comment, the
  convention of the Action pins. Renovate reads the tag as the current value
  and the commit as its digest through the GitHub tags datasource. The
  installer SHA-256 is read from the release's `scripts/install.sh` at that
  commit; Renovate does not compute it.
- The test exports `nativeClients` with `cue` and applies the policy's regex
  rules to the CUE file with Node, parsing the policy as the existing Renovate
  ownership test does, so the rules are tested as Renovate reads them.
- Claude Code 2.1.317 waits for the age of its last supplying platform package
  (`win32-x64`, published 2026-11-03T00:04Z), as the supply-chain
  specification requires.
- uv 0.13.7 does not wait for the 24-hour default age that Mise applies when it
  resolves a version. The policy admits checksum-locked tools without an age,
  and a security fix moves a locked tool to its first fixed release; the lock
  records GitHub attestation provenance for every platform.

## Risks / Trade-offs

- Hermes v0.22.6 rolls up 776 merged pull requests since v0.22.5; its
  installer differs only in help text. The native client journeys on macOS,
  Linux and Windows exercise the release before the forges integrate it. A
  failure there keeps the pin at v0.22.5 rather than adapting AIGW in this
  Change.
- A tag in a comment can drift from its commit. The tag only names the release
  for the candidate calculation; the supply installs the pinned commit and
  verifies the installer's SHA-256 at that commit.
