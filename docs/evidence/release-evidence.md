# Release Evidence

This document maps each release claim to current evidence. It never substitutes
an old log, branch name, runner incident, tag, or signing identity for proof.

## Claims and evidence

| Claim | Required current evidence | Insufficient evidence |
| --- | --- | --- |
| Source is releasable | Clean target revision; aggregate statement and branch coverage each strictly above 95 percent with exact raw counts; every package present, executed, and reported; race detection; static analysis; provider commit verification | An old log, excluded or wholly unexecuted package, statement-only evidence, or aggregate evidence without package observation |
| Artifact matrix is complete | Two builds from one version, epoch, Go toolchain, and explicit Forge coordinates; byte-identical six platform archives, SPDX SBOM, and checksum manifest | A partial set, implicit deployment tuple, or semantically similar bytes |
| Installation works | Native macOS, Linux, and Windows execution of `aigw install`, update, rollback, and uninstall against the candidate archive | Cross-compilation or archive inspection alone |
| Windows runtime works | Blocking managed Windows build, install, execution, upgrade, rollback, and uninstall evidence | Non-Windows PowerShell syntax or an allowed failure |
| Release is published | Successful tag pipeline and release inspection on that Forge; when both releases exist, independent inspection proves matching tag, assets, checksums, and SBOM | A local output directory or source tag |
| GA is trusted | Protected CI verification of the chosen signing policy and post-signature checksums for the exact assets | An unsigned prerelease or local identity inspection |

## Prerelease and GA boundary

Prerelease versions may publish checksum-verified archives and an SPDX SBOM.
They must not claim unavailable signing or notarization. Native macOS, Linux,
and Windows source and lifecycle acceptance block every release; missing runner
capacity or infrastructure failure blocks publication rather than weakening the
gate. GA additionally requires protected signing evidence for the exact
published assets.

## Release sequence

1. Freeze one clean candidate revision and record its SHA.
2. Derive the release epoch from that revision's Changelog heading.
3. Run all source, coverage, static-analysis, provenance, and native-platform
   gates.
4. Build the eight-asset matrix twice with `release`; require identical names
   and bytes.
5. Validate checksums, SBOM portability, archive layout, and native lifecycle
   evidence.
6. Create an SSH-signed annotated tag for the exact revision.
7. Let each Forge independently verify, build, publish, and inspect its release.
8. Install a published archive in a clean environment; prove one update and
   rollback.
9. Create a GA tag only when the protected signing policy applies to those exact
   assets.

A source tag proves source identity only. It does not prove publication,
installation, platform acceptance, or GA trust.
