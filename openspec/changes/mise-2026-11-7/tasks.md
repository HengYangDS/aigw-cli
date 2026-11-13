## 1. Declaration

- [x] 1.1 Pin Mise 2026.11.7 in the CUE authority: the Linux image index digest
      and the Windows executable and shim digests;
      `go run ./tools/ci project --check` reports no drift, and the generated
      GitHub workflows install 2026.11.7.
- [x] 1.2 Add the macOS runner-owned Mise preflight to the four macOS GitLab
      jobs and record its text in `.config/ci/retained-steps.toml`;
      `TestGitLabMacOSVerifiesRunnerOwnedMiseBeforeRepositoryTools` fails when
      a job lacks it and passes with it.

## 2. Hosts

- [ ] 2.1 Install the verified 2026.11.7 executable as
      `/opt/mise-aigw-cli/bin/mise` on the GitLab macOS ARM64 host, root-owned
      and not writable by the runner accounts, in a window without AIGW jobs;
      its digest and version match the CUE pins.
- [ ] 2.2 Replace `mise.exe` and `mise-shim.exe` in
      `C:\Program Files\mise-aigw-cli\bin` on the GitLab Windows ARM64 host with
      the verified 2026.11.7 files in the same window, keeping the previous
      files as rollback; both digests match the CUE pins, and the ACL is
      unchanged.

## 3. Verification

- [ ] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head.
- [ ] 3.2 The hosted graph passes at the proposal head, with every GitLab macOS
      and Windows job reporting Mise 2026.11.7.
