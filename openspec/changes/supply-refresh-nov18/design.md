## Context

The sweep's sources: `mise latest` for every tool of `.config/mise/config.toml` (go, node, cue, GoReleaser,
apple-codesign, Taplo, scc, editorconfig-checker, gitleaks, golangci-lint, ShellCheck, actionlint, Hyperfine,
typos, lychee and OSV Scanner are current); `go list -m -u all` (one direct requirement behind; the others are
indirect); the npm registry for the four direct npm packages (OpenSpec behind); GitHub releases of Renovate,
`jdx/mise-action`, `actions/checkout` and `actions/upload-artifact` (Renovate behind); the npm registry for Codex
and Claude Code and GitHub's releases of Hermes for the native clients (all three behind). Hermes v0.23.2's
installer digest is the SHA-256 of `scripts/install.sh` at its release commit, read the way that reproduces
v0.23.1's pinned digest.

## Goals / Non-Goals

**Goals:**

- Every declared input at its latest stable release, except the one held below.

**Non-Goals:**

- Indirect Go requirements that no direct requirement asks for.
- A change of a supply rule.

## Decisions

### Renovate takes the newest release that is 72 hours old

- **Rests on:** the owner's word of 2026-11-18 evening, relayed by the host supply-chain session and confirmed
  first-hand in this lane's session: the host keeps one Renovate image, and this lane's Renovate goes back to a
  release that is 72 hours old, while its other supply moves stay. The owner set a 72-hour release age that
  afternoon. 44.251.0, taken at 11:55, was published on 2026-11-17 at 13:50Z.
- **Considered:** keep 44.251.0; wait until it is 72 hours old (2026-11-20); move back every input of this
  Change that is under 72 hours old; move back Renovate alone.
- **Chosen because:** the owner chose Renovate alone. 44.239.1 (2026-11-13 18:47Z) is the newest release at least
  72 hours old. It is older than dev's 44.242.0; the release age, not the version order, decides.
- **Applies to:** `vars.renovate_image` in `.config/mise/config.toml`. The repository's own Renovate policy keeps its
  declared age until the lane that sets 72 hours.

### The hosted Mise waits for the runners' window

- **Rests on:** the refresh of 2026-11-16: the CUE authority derives one Mise release from the Linux image's tag
  and pins the digests of the macOS and Windows executables that the GitLab runners own; each job verifies its
  executable against them.
- **Considered:** move the image alone; hold the window today.
- **Chosen because:** the image cannot move alone, and the window for the runners is set for 2026-11-20.
- **Applies to:** the Mise image, the runner-owned executables and `jdx/mise-action`'s Mise version. Owner: the
  maintainer of this repository's CI guests. Date: 2026-11-20.

## Risks / Trade-offs

- **A tool's new release changes a check's output.** The full check runs on the lane head before the proof.
