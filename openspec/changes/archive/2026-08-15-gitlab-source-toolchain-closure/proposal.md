## Why

GitLab source verification currently asks mise to install the repository's
entire tool inventory. That installs release-only tools and downloads five
GitHub-hosted binaries before any source gate can run. The resulting timeout is
a CI topology defect, not a reason to increase network timeouts.

## What Changes

- Declare the source job's exact mise tool closure.
- Mirror only the additional source tools into the existing GitLab Generic
  Package Registry.
- Bind the mirror package to `mise.lock` and verify its manifest and assets.
- Keep native and release jobs on their own smaller tool closures.

## Non-goals

- Windows runner host repair is operational work outside this source Change.
- GitHub remains an independent hosted plane.
- Release packaging and product behavior do not change.
