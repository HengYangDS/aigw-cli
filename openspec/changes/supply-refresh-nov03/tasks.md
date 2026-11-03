## 1. Refresh

- [x] 1.1 Advance the Renovate image to 44.202.1, pinned to its index digest,
      and glab to 1.127.0 with `mise lock` entries for all six platforms, and
      refresh the npm lock within its declared ranges under the three-day
      release age; registry signatures verify and `npm audit` reports no
      vulnerability.
- [x] 1.2 Once their release ages end, advance OpenSpec to 1.16.0 and the
      Claude Code pin to 2.1.316, or to a newer stable release that the policy
      then admits; both projection tests and the Windows client supplier name
      follow.
- [x] 1.3 Advance the releases that the morning candidate calculation admits:
      the Renovate image to 44.206.2, pinned to its index digest, syft to
      1.56.0 and typos to 1.52.0 with `mise lock` entries for all six
      platforms, and mise-action to v5.2.6, pinned to its commit;
      `tools/ci project --check` reports no drift.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on each lane
      head that carries this refresh.
- [ ] 2.2 The hosted graph passes at each such head, and the native client
      journeys on macOS, Linux and Windows pass with the advanced client pin.
