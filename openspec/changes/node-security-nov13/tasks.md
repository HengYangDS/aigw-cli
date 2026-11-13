## 1. Refresh

- [x] 1.1 Advance Node to 26.13.1 in `.config/mise/config.toml` and in
      `.config/mise/mise.lock` for all six platforms; each checksum equals the
      release's `SHASUMS256.txt`. Done in `10f7d7ca`.
- [x] 1.2 Advance gh, glab, goreleaser, OSV Scanner, Syft, typos and uv to
      their newest stable releases with `mise lock` entries for all six
      platforms; each changed checksum equals its release's published list, and
      `mise install --locked` succeeds. All 42 entries match; typos publishes no
      list, so its entries match GitHub's asset digests.
- [x] 1.3 Advance `charm.land/lipgloss/v2` to 2.1.0, `golang.org/x/crypto` to
      0.59.0 and `golang.org/x/term` to 0.48.0; `go mod tidy` leaves no
      difference.
- [x] 1.4 Advance markdownlint to 0.42.1 and the jsdom override to 31.0.0;
      `npm ci` succeeds, and the Markdown and Mermaid gates pass. Done in
      `4cc29c6b`: the check runs both gates, and each Forge's quality job
      installs the lock with `npm ci`.
- [x] 1.5 Advance mise-action to 5.2.8 and Hermes to v0.23.1 in the CUE
      authority; `go run ./tools/ci project --check` reports no drift.
- [x] 1.6 Advance the Renovate image to 44.236.5, pinned to its index digest,
      with `dependencyDashboard` in its migrated object form;
      `mise run dependencies:check` passes.
- [x] 1.7 Make the Hermes candidate rule accept release tags only; its test
      fails while the rule admits `v0.23.1+canary.20261110T064422Z`, and the
      candidate calculation proposes v0.23.1.
      `TestRenovateReadsOnlyReleaseTagsOfHermes` fails with `semver-coerced`
      and passes with the rule; on the old pin the calculation proposes v0.23.1.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head (`4cc29c6b`, statement coverage 95.55%).
- [x] 2.2 The hosted graph passes at the proposal head (`4cc29c6b`: GitLab
      MR !191 pipeline 15247, GitHub PR 257 run 43352688345).
