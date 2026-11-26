## 1. Declaration

- [x] 1.1 Pin Mise 2026.11.14 in the CUE authority: the Linux image by its index
      digest and the macOS runner executable by its `SHASUMS256.txt` digest;
      `go run ./tools/ci project --check` reports no drift, and the generated
      GitHub workflows install 2026.11.14.
- [x] 1.2 Derive the macOS runner Mise directory from the declared release and
      record the new preflight text in `.config/ci/retained-steps.toml`;
      the GitLab macOS projection tests fail when the directory does not name
      the declared release and pass with it.
- [x] 1.3 Pin jdx/mise-action v5.3.0 by its commit in the CUE authority; the
      generated GitHub workflows use that commit.
- [x] 1.4 Run a locked install under Mise 2026.11.14 with new data, cache, state
      and configuration directories; every locked tool installs and
      `.config/mise/mise.lock` is unchanged.
- [x] 1.5 Describe the release-named directory and its cutover in the
      dependency maintenance guide and record the move in `CHANGELOG.md`; the
      prose and Markdown gates pass.

## 2. Host

- [x] 2.1 Install the verified 2026.11.14 executable as
      `/opt/mise-aigw-cli/2026.11.14/bin/mise` on the GitLab macOS ARM64 host,
      beside the earlier release, root-owned and not writable by the runner
      accounts, before the proposal's first GitLab pipeline; under both AIGW
      runner accounts, `ci-aigw-review` and `ci-aigw-protected`, the path's
      `shasum -a 256` and `--version` equal the CUE pins. Owner:
      `ETHOS 仓库纳管`. Done 2026-11-26 06:35, by that session's relayed
      reading (design.md, Verification of the new release).

## 3. Verification

- [x] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head. `7f7b0c060`, 2026-11-26 08:14: both pass; statement coverage 95.85%.
- [x] 3.2 The hosted graph passes at the proposal head, where
      `native-darwin-review` is the only macOS job and runs under the review
      account; it prints the verified Mise 2026.11.14. `6b33acc50`, 2026-11-26:
      pull request 287 (4 passed, 2 skipped) and GitLab pipeline 18261 (10:41;
      `Runner-owned Mise verified: 2026.11.14 macos-arm64 (2026-11-24)`).

## Workflow follow-up

- The protected account's macOS jobs print the verified Mise 2026.11.14 where
  they first run after landing: `native-darwin` in the dev push pipeline,
  `accepted-ref-parity` in the main push pipeline and `release-version` in the
  next tag pipeline.
- `ETHOS 仓库纳管` removes `/opt/mise-aigw-cli/bin` (owner on the
  coordinator's word; review date 2026-12-03) once, after a fetch,
  `git merge-base --is-ancestor <landed commit> gitlab/dev` and the same check
  against `gitlab/main` succeed, and the open merge requests (`glab mr list`)
  hold no proposal without the landed commit. Until it carries this Change, a
  ref still verifies the earlier path and fails at its preflight after the
  removal: every tag of an earlier commit and a retry of its pipeline, open
  proposals, and unpublished lanes such as `20261125-program-truth` at
  `4c76b25cb`. For a later release, the step removes the whole
  `/opt/mise-aigw-cli/<earlier release>` directory (design.md, Migration Plan).
