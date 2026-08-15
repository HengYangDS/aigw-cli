## 1. Define the closure

- [x] 1.1 Add a failing projection test for the exact source tool closure.
- [x] 1.2 Add the source-specific CUE template and regenerate GitLab CI.

## 2. Bind the mirror

- [x] 2.1 Verify upstream archives against `mise.lock`.
- [x] 2.2 Publish the verified archives under the `mise.lock` digest.
- [x] 2.3 Redirect locked artifact URLs so mise verifies and installs every archive.

## 3. Prove delivery

- [x] 3.1 Pass focused projection tests and the complete source gate.
- [x] 3.2 Commit and pass exact-HEAD proof.

## Delivery Boundary

Archive, candidate integration, accepted closeout, peer publication, hosted CI,
release, runtime installation, and lane retirement are lifecycle effects proven
by their own receipts after this Change is complete.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `ci-diagnostics:Hosted Git initialization is explicit` | `1.1` | focused red/green `TestGitLabSourceJobUsesItsExactToolClosure` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `1.2` | exact CUE projection and generated `.gitlab-ci.yml` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `2.1` | upstream archive SHA-256 equals `mise.lock` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `2.2` | GitLab package `ci-source-tools/<mise.lock-sha256>` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `2.3` | mise locked install with project-local URL replacement |
| `ci-diagnostics:Hosted Git initialization is explicit` | `3.1` | `mise exec --locked -- go run ./tools/ci source` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `3.2` | exact-HEAD ETHOS proof `24d530bfd4d476a77c4531bbb183354d525e6a53bf438228d04cf13e3ab6e7cb` |
