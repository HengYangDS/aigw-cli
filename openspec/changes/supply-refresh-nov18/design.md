## Context

The sweep's sources: `mise latest` for every tool of `.config/mise/config.toml` (go, node, cue, GoReleaser,
apple-codesign, taplo, scc, editorconfig-checker, gitleaks, golangci-lint, ShellCheck, actionlint, Hyperfine,
typos, lychee and OSV Scanner are current); `go list -m -u all` (one direct requirement behind; the others are
indirect); the npm registry for the four direct npm packages (OpenSpec behind); GitHub releases of Renovate,
`jdx/mise-action`, `actions/checkout` and `actions/upload-artifact` (Renovate behind).

## Goals / Non-Goals

**Goals:**

- Every declared input at its latest stable release, except the one held below.

**Non-Goals:**

- Indirect Go requirements that no direct requirement asks for.
- A change of a supply rule.

## Decisions

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
