# Stable macOS release and Homebrew distribution

## Intent and boundaries

Deliver an installable public AIGW release without replacing the working installation before exact-artifact acceptance. The next target is `0.1.0`, not another RC; a 0.x version remains an initial-development API line rather than a promise of 1.x compatibility. Product readiness, publisher trust, transport authorization, and credential access are independent facts.

## Ownership

- Existing release construction owns reproducible source/toolchain inputs and the unsigned or ad-hoc-qualified matrix.
- Existing release tooling owns one bounded macOS distribution transformation using native signing/notarization capabilities. It records the source artifact, final bytes, signer, timestamp, and Apple submission result.
- Existing inventory, manifest signature, checksums, and publisher own the final immutable assets. Signing happens before final inventory construction. Both peers receive the same bytes; existing RC assets are never replaced.
- GoReleaser and a minimal Homebrew tap own packaging projections. macOS packaging must preserve the publisher-signed executable. Linux packaging uses its declared portable build; Windows retains the existing archive channel.
- The detected installation owner controls program replacement. Homebrew-managed installs direct upgrade/uninstall to Homebrew; portable installs retain the existing lifecycle.
- User-triggered setup owns configuration. Package installation neither reads Tokens nor changes client settings. Removal must not leave active credential commands pointing at a deleted executable; preparation and recovery are explicit while credentials remain preserved.

## Choices

Prefer native macOS signing with the existing login-Keychain identity for the initial operator-driven release, avoiding private-key export merely to enable CI. Evaluate GoReleaser's native signing integration before adding an adapter. Notarization uses a supported Apple credential stored by the native tool; individual developer membership is not confused with API-key type.

Use a self-maintained tap first. Official core submission is not a prerequisite. Decide Formula versus Cask from artifact and ownership requirements, not the executable being a CLI. Do not create parallel source and binary distributions unless their consumers and acceptance are explicit.

Keep reproducibility claims scoped to construction inputs. Trusted signing timestamps and notarization are external observations, not deterministic rebuild output. Do not suppress certificate or Gatekeeper verification to preserve an obsolete byte-equality claim.

## Execution order

First qualify local identity and bounded signing; then implement final-byte distribution and tests. Admit stable versions through the existing acceptance owner. Only then publish and generate the tap from verified inventory. Run clean installation and retained-state upgrade/rollback before the local cutover. Complete peer parity, documentation, and cleanup last. Missing Apple authorization blocks notarization claims only, not independent implementation or validation.

## Risks and recovery

A signing identity listed by Keychain does not prove usable noninteractive access. A notary submission is not acceptance. Failure keeps the current release and credentials intact. Pending submission handles are resumed, not recreated. Homebrew replacement must retain clients' usable executable path or explicitly prepare their projections; binary removal alone is not a complete product uninstall.
