## Context

The 2026-11-04 candidate calculation of the repository's Renovate policy found
four newer stable releases. Three advance unchanged. The fourth, Mise
2026.11.0, changes how safe mode treats project lockfiles (upstream mise issue
15235). In safe and locked mode, it no longer trusts the project lockfile, so
`mise install --locked` refuses tools whose locked entry it cannot reproduce
without that trust. For `npm:npm@12.2.1` it reports "no embedded-aube
dependency graph in the revision 2 lockfile", although aigw's lockfile is
format 3. In a fresh data directory, core tools such as Node fail as well, with
"No lockfile URL found".

The 2026-11-04 rehearsals used the official 2026.11.0 binary in isolated
directories:

- Locked mode alone installs all 21 tools.
- Safe mode without locked mode reports success, but npm 12.2.1 stays missing,
  so Node's bundled npm 11.20.0 runs instead.

ETHOS resolves repository toolchains in safe and locked mode. That mode keeps
an adopter's Mise configuration inert during resolution.

## Goals / Non-Goals

**Goals:**

- Take every stable release whose admission evidence passes.
- Keep a release that breaks the repository's own verification held under a
  named owner and exit condition.

**Non-Goals:**

- Change Mise's safe or locked mode, or drop either to make a release pass.
- Patch Mise or ETHOS.

## Decisions

- One CUE value, `miseImage`, selects the Mise release. The GitLab image, the
  GitHub mise-action version and the Windows runner-owned executable check all
  derive from it. A partial move would split the projections, so the GitLab
  image stays at 2026.10.25 together with the others. The alternative, moving
  only the GitLab image, was rejected: GitHub and Windows would then verify
  against different Mise releases.
- The hold has an owner and an exit condition. The host's Homebrew Mise pin
  belongs to the ETHOS session that set it. The exit condition is that the
  safe and locked resolution of aigw's toolchain passes in a fresh data
  directory under Mise 2026.11.0 or later, which needs an upstream Mise fix.
  aigw-cli is in that fix's acceptance set together with
  architecture-publisher and web-ingestion-benchmark.
- Disabling locked mode is not a mitigation. It reports success while the
  declared tool is missing, which weakens supply admission without failing.

## Risks / Trade-offs

- [Mise falls behind its newest stable release] → The hold is bounded by the
  exit condition and reviewed on 2026-11-06.
- [The Hermes release rolls up about 174 merged PRs] → The native client
  journeys on macOS, Linux and Windows must pass with v0.22.7 before the forges
  integrate it.
