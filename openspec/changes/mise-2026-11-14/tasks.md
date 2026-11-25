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

- [ ] 2.1 Install the verified 2026.11.14 executable as
      `/opt/mise-aigw-cli/2026.11.14/bin/mise` on the GitLab macOS ARM64 host,
      beside the earlier release, root-owned and not writable by the runner
      accounts, before the proposal's first GitLab pipeline; its digest and
      version match the CUE pins. Owner: the owner of the CI hosts.

## 3. Verification

- [ ] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head.
- [ ] 3.2 The hosted graph passes at the proposal head, with every GitLab macOS
      job printing the verified Mise 2026.11.14.

## Workflow follow-up

- Once dev and main carry this Change and no open proposal runs an earlier
  declaration, the owner of the CI hosts removes `/opt/mise-aigw-cli/bin`
  (design.md, Migration Plan).
