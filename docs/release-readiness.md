# Release Evidence Contract

This document defines the evidence required to make a release claim. It does
not record a particular commit, RC version, branch, GitLab outage, or signing
identity snapshot. Those facts change; obtain them from the current checkout,
CI pipeline, GitLab Release, and signed artifacts at release time.

## Release claims and their proof

| Claim | Required current evidence | Not sufficient |
|---|---|---|
| Local source is ready to package | Clean target revision; `go test -race ./...`; `go vet ./...`; release gate scripts | An earlier terminal log or a green result from another commit |
| RC artifact matrix is complete | `AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh <pre-release-version> dist`; `check-release-artifacts.sh`; `test-release-package-layout.sh` | A subset of archives or build success without checksums |
| Portable installation works | Unix installer test plus PowerShell installer test against the candidate binary | Static script review alone |
| Linux package path works | `test-linux-native-install.sh dist <version>` on its declared compatible image, or stronger native distribution-runner evidence | Cross-compilation, archive inspection, or a failed/unavailable container run |
| Windows installer behavior works | PowerShell harness run under PowerShell; native Windows runner evidence is stronger when available | MSI metadata inspection alone |
| Release is remotely published | GitLab package upload and Release job for the exact tag; inspect the resulting Release assets and checksums | A locally created `dist/` directory |
| GA is signed and trusted | Protected CI verifies macOS Developer ID + notarization/stapling and Windows Authenticode + timestamp for the exact published assets | An unsigned RC, a local identity check, or a manually uploaded asset |

No completed local check implies remote publication. No remote upload implies a
signed GA. State each claim only to the strength of its corresponding current
evidence.

## RC and GA boundary

`scripts/check-release-readiness.sh` enforces the release class:

- `*-rc.*`, `*-beta.*`, and `*-alpha.*` may be packaged as pre-releases with
  checksums and SPDX SBOM evidence. They must not claim signing or notarization.
- A version without a pre-release suffix is blocked until protected CI contains
  and verifies all production signing work. No environment variable, manual
  upload, or local workaround may bypass this gate.

GA protected CI must prove, for the exact release assets:

1. macOS Developer ID signing for the binary and package, notarization, and
   stapling, followed by independent verification;
2. Windows Authenticode signing and timestamp verification on a managed Windows
   runner;
3. checksums regenerated after signing, then verified before publish/release;
4. signing credentials sourced only from protected/masked CI variables, a
   runner keychain, or an organization key service.

## Cross-platform installation evidence

The package script emits portable archives for macOS/Linux/Windows `amd64` and
`arm64`, a macOS Universal `.pkg`, Linux `.deb`/`.rpm`, Windows `.msi`,
checksums, and an SPDX SBOM. `check-release-artifacts.sh` and
`test-release-package-layout.sh` verify that matrix structurally.

Structural evidence is deliberately different from runtime evidence. The Linux
acceptance script installs the `amd64` `.deb` and `.rpm` in an Alpine x86_64
compatibility container, then runs `/usr/bin/aigw`. Its architecture-name
workaround is documented in the script and does not establish Debian/Fedora
native acceptance. A managed Debian runner and a managed Fedora runner remain
the stronger evidence for those distributions. If the image, engine, or network
is unavailable, record the Linux runtime evidence as unavailable rather than
reusing an older result.

## Release procedure

1. Start from the intended clean revision; record its SHA.
2. Run the full verification suite and package the exact pre-release version.
3. Verify checksums, SBOM, package layout, installer behavior, and any available
   Linux/Windows native-runtime evidence against that exact `dist/` directory.
4. Push the reviewed branch, merge through the protected default-branch flow,
   then create a pre-release tag from the merged commit.
5. Inspect the exact GitLab Release and package assets after the CI publish and
   release jobs succeed. Perform a clean-environment install only from those
   remotely published artifacts before describing the RC as distributable.
6. Create a GA tag only after the protected signing evidence above is present
   and verified for the published asset set.

Network availability, CI runner capacity, signing identities, and GitLab state
are runtime conditions. Diagnose and report them at the time of release; do not
encode a transient result in this contract.
