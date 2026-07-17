# Release Evidence

This document defines which current evidence supports which release claim. It
does not record a historical branch, runner incident, tag, or signing identity.

## Claims and evidence

| Claim | Required current evidence | Insufficient evidence |
| --- | --- | --- |
| Source is packageable | Clean target revision, `go test -race ./...`, `go vet ./...`, and all release gates | An old terminal log |
| RC artifact matrix is complete | Two full builds on the dedicated release runner, with one exact version, epoch, Go patch version from `go.mod`, tracked forge-source manifest, byte-identical 15-artifact matrices, artifact check, and package-layout check | A partial archive set, a provider-specific build tuple, or semantically similar but byte-different artifacts |
| Portable installation works | Unix and PowerShell installer tests against the candidate binary | Static script review |
| Linux native package path works | Isolated Debian and RPM-family installation evidence for both architectures, or stronger native-runner proof | Cross-compilation alone |
| Windows RC assurance | Cross-compiled executables, MSI/ZIP layout and architecture checks, and a real PowerShell installer contract | Cross-compilation alone or MSI metadata alone |
| Windows native runtime works | Managed Windows-runner package, install, upgrade, uninstall, PATH, shim, and execution evidence | Non-Windows PowerShell syntax or structural package checks |
| macOS native package lifecycle works | Rooted installation, upgrade, execution under an isolated local account, package receipt, and owned uninstall evidence on a disposable APFS volume | Package expansion or a portable-archive test |
| Release is published | Successful tag pipeline upload and release inspection on the publishing forge; when both forge releases are present, independent GitLab/GitHub inspection proves matching tag, assets, checksums, and SBOM | A local `dist/` directory or source tag |
| GA is trusted | Protected CI verification of actual signed/notarized macOS and Windows assets | An unsigned RC or local identity inspection |

## RC and GA boundary

Prerelease versions (`-rc`, `-beta`, `-alpha`) may publish checksum-verified
artifacts and an SPDX SBOM. They must not claim signing or notarization. A
managed Windows runner is supplementary RC evidence: its absence or runner
infrastructure failure does not block an RC after the mandatory Windows RC
assurance gate passes. The disposable-volume macOS native lifecycle proof is
also additive until a protected release runner has an approved dedicated
credential. Both native proofs remain mandatory before GA, together with
protected CI verification of Developer ID signing, notarization/stapling,
Windows Authenticode/time-stamping, and post-signature checksums for the exact
published assets.

## Release sequence

1. Start from a clean candidate revision and record its SHA.
2. Derive `SOURCE_DATE_EPOCH` from the exact candidate's committed Changelog
   heading, select the Go patch version declared in `go.mod` and tracked forge-source manifest, run
   the full gate set, and build the candidate matrix twice on the dedicated
   macOS arm64 release runner.
3. Require identical filenames and bytes across both matrices, then validate
   checksums, SBOM, layout, installers, and available native evidence against
   one retained `dist/` directory.
4. Create an SSH-signed annotated prerelease tag for that exact revision. The
   tag pipeline verifies the repository-owned signer anchor before packaging.
5. Merge only that candidate into the protected default branch and confirm that
   the tag is an ancestor of `main`.
6. Confirm remote package upload and release assets on both publishing forges;
   inspect both tags, asset names, checksums, and SBOMs for exact agreement
   before offering dual-forge updates.
7. Perform a clean-environment installation from published assets and one
   explicit offline `aigw update --candidate ... --checksums ...` update plus
   portable rollback proof.
8. Create a GA tag only after the protected signing evidence applies to the
   exact published artifacts.

A source tag records source identity; it is never, by itself, proof of
publication, installation, platform acceptance, signing, or GA status.
