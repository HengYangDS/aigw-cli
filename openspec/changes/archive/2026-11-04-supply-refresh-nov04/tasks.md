## 1. Refresh

- [x] 1.1 Advance the Renovate image to 44.210.0, pinned to its index digest,
      and glab to 1.128.0, with `mise lock` entries for all six platforms;
      `tools/ci project --check` reports no drift. Done in `e3e687e0`: each
      glab checksum equals the release's published `checksums.txt`.
- [x] 1.2 Advance the Hermes native client pin to v0.22.7: its tag commit and
      the SHA-256 of its `scripts/install.sh`; the projection test and the
      GitHub projection follow. Done in `e3e687e0`: the installer's SHA-256 is
      unchanged from v0.22.6.
- [x] 1.3 Keep Mise at 2026.10.25 on every projection, and record the hold's
      owner and exit condition in the design: the safe and locked resolution
      of aigw's toolchain must pass in a fresh data directory under Mise
      2026.11.0 or later.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head that carries this refresh (`e3e687e0`, statement coverage 95.22%).
- [x] 2.2 The hosted graph passes at the proposal head, and the native client
      journeys on macOS, Linux and Windows pass with Hermes v0.22.7 (PR 234 run
      41775030404; journeys run 41778668200, from v0.5.0).
