## 1. Establish immutable GitLab bootstrap assets

- [x] 1.1 Verify the latest stable mise release and both Linux asset digests.
- [x] 1.2 Mirror the verified assets to AIGW's GitLab Generic Package Registry.
- [x] 1.3 Download the mirrored assets and prove byte-for-byte digest parity.

## 2. Replace duplicated bootstrap logic

- [x] 2.1 Add a failing projection regression for one inherited Linux toolchain template.
- [x] 2.2 Implement the CUE template and remove job-local bootstrap scripts.
- [x] 2.3 Raise the repository mise minimum to the verified latest stable release.
- [x] 2.4 Regenerate `.gitlab-ci.yml` from the CUE SSOT.

## 3. Verify and deliver

- [x] 3.1 Pass focused projection and CI package tests.
- [x] 3.2 Pass the complete local source gate.
- [x] 3.3 Commit the final source tree and pass exact-HEAD proof.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `ci-diagnostics:Hosted Git initialization is explicit` | `2.2` | `tools/ci/projection_test.go:TestGitLabLinuxJobsInheritOneProjectLocalToolchainBootstrap` |
