## Why

Go advisory GO-2026-6809 (CVE-2026-111943, published 2026-10-21) reports that
`crypto/x509` before Go 1.27.3 still trusts intermediate certificates whose
private keys were compromised; Go 1.27.3, released 2026-10-19, distrusts them.
AIGW locks Go 1.27.2, so every AIGW program built from the lock, including the
released 0.4.9 binaries, verifies TLS through the vulnerable package.

A Renovate `dryRun=full` candidate calculation with the repository policy on
GitHub `dev` (`ec6ddcdc`, 2026-10-21) also admits Syft 1.55.0, which remediates
five vulnerabilities in its own dependencies, gh 2.102.1, glab 1.124.0 and the
Renovate image 44.173.0.

## What Changes

- Advance the locked Go toolchain, the `go.mod` directive and the version-bound
  standard-library scan override from 1.27.2 to 1.27.3.
- Advance Syft to 1.55.0, gh to 2.102.1, glab to 1.124.0 and the Renovate image
  to 44.173.0, and regenerate their Mise lock entries with `mise lock`.
- Hold two admissible updates with named owners: Mise 2026.10.17, whose Windows
  executable the GitLab runner owns and GitLab must accept after its
  replacement, and the Windows native client pins Codex 0.163.0 and Claude Code
  2.1.302, which each need their own native client journey.
- The supply-chain requirement gains a scenario: an advisory against the
  toolchain compiled into released programs requires a new release.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: the latest-stable supply-chain requirement gains a
  scenario for a security advisory against the toolchain compiled into
  released programs.

## Impact

Dependency declarations, the Mise lock, `go.mod` and the dependency policy
change. AIGW programs built afterwards carry the Go 1.27.3 standard library;
installed AIGW, user configuration and credentials change only when the next
release is installed.
