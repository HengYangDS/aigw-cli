## 1. Define the closure

- [x] 1.1 Add a failing projection test for the exact source tool closure.
- [x] 1.2 Add the source-specific CUE template and regenerate GitLab CI.

## 2. Bind the mirror

- [x] 2.1 Verify upstream archives against `mise.lock`.
- [x] 2.2 Publish the verified archives under the `mise.lock` digest.
- [x] 2.3 Redirect locked artifact URLs so mise verifies and installs every archive.

## 3. Prove delivery

- [x] 3.1 Pass focused projection tests and the complete source gate.
- [ ] 3.2 Commit and pass exact-HEAD proof.
- [ ] 3.3 Land and prove GitLab source verification on `dev` and `main`.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `ci-diagnostics:Hosted Git initialization is explicit` | `1.1` | focused red/green `TestGitLabSourceJobUsesItsExactToolClosure` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `1.2` | exact CUE projection and generated `.gitlab-ci.yml` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `2.1` | upstream archive SHA-256 equals `mise.lock` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `2.2` | GitLab package `ci-source-tools/<mise.lock-sha256>` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `2.3` | mise locked install with project-local URL replacement |
| `ci-diagnostics:Hosted Git initialization is explicit` | `3.1` | `mise exec --locked -- go run ./tools/ci source` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `3.2` | exact-HEAD ETHOS proof |
| `ci-diagnostics:Hosted Git initialization is explicit` | `3.3` | hosted GitLab `source-and-governance` on `dev` and `main` |
