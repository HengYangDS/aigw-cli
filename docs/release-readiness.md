# Release Evidence

This document defines which current evidence supports which release claim. It
does not record a historical branch, runner incident, tag, or signing identity.

## Claims and evidence

| Claim | Required current evidence | Insufficient evidence |
| --- | --- | --- |
| Source is packageable | Clean target revision, `go test -race ./...`, `go vet ./...`, and all release gates | An old terminal log |
| RC artifact matrix is complete | Full package build, artifact check, and package-layout check for one exact version | A partial archive set |
| Portable installation works | Unix and PowerShell installer tests against the candidate binary | Static script review |
| Linux native package path works | Isolated Debian and RPM-family installation evidence for both architectures, or stronger native-runner proof | Cross-compilation alone |
| Windows installer works | Managed Windows-runner package and runtime evidence | MSI metadata or non-Windows PowerShell syntax |
| Release is published | Successful tag pipeline upload and GitLab Release inspection; when enabled, GitHub mirror inspection proves identical tag, assets, checksums, and SBOM | A local `dist/` directory or source tag |
| GA is trusted | Protected CI verification of actual signed/notarized macOS and Windows assets | An unsigned RC or local identity inspection |

## RC and GA boundary

Prerelease versions (`-rc`, `-beta`, `-alpha`) may publish checksum-verified
artifacts and an SPDX SBOM. They must not claim signing or notarization. A GA
version fails closed until protected CI verifies Developer ID signing,
notarization/stapling, Windows Authenticode/time-stamping, and post-signature
checksums for the exact published assets.

## Release sequence

1. Start from a clean candidate revision and record its SHA.
2. Run the full gate set and build the exact candidate version.
3. Validate checksums, SBOM, layout, installers, and available native evidence
   against the same `dist/` directory.
4. Create an SSH-signed annotated prerelease tag for that exact revision. The
   tag pipeline verifies the repository-owned signer anchor before packaging.
5. Merge only that candidate into the protected default branch and confirm that
   the tag is an ancestor of `main`.
6. Confirm remote package upload and GitLab Release assets. When the GitHub
   mirror is enabled, inspect its tag, asset names, checksums, and SBOM against
   the GitLab release before treating it as an update fallback.
7. Perform a clean-environment installation from published assets and one
   offline verified-candidate update plus portable rollback proof.
8. Create a GA tag only after the protected signing evidence applies to the
   exact published artifacts.

A source tag records source identity; it is never, by itself, proof of
publication, installation, platform acceptance, signing, or GA status.
