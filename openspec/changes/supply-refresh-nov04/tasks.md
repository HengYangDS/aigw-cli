## 1. Refresh

- [ ] 1.1 Advance the Renovate image to 44.210.0, pinned to its index digest,
      and glab to 1.128.0, with `mise lock` entries for all six platforms;
      `tools/ci project --check` reports no drift.
- [ ] 1.2 Advance the Hermes native client pin to v0.22.7: its tag commit and
      the SHA-256 of its `scripts/install.sh`. Both projection tests and the
      Windows client supplier name follow.
- [ ] 1.3 Keep Mise at 2026.10.25 on every projection, and record the hold's
      owner and exit condition in the design: the safe and locked resolution
      of aigw's toolchain must pass in a fresh data directory under Mise
      2026.11.0 or later.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head that carries this refresh.
- [ ] 2.2 The hosted graph passes at the landed head, and the native client
      journeys on macOS, Linux and Windows pass with Hermes v0.22.7.
