## 1. Contract and failing tests

- [x] 1.1 Define the portable selection and secure-file behavior and verify `openspec validate portable-secret-backend --strict` passes
- [x] 1.2 Add failing platform-path and backend-selection tests covering macOS, Linux, Windows, explicit keyring, explicit env, and stable automatic selection
- [x] 1.3 Add failing file-store tests covering atomic CRUD and rejection of unsafe permissions, symbolic links, hard links, and foreign ownership where observable

## 2. Product implementation

- [x] 2.1 Add the AIGW-owned secrets path and verify platform path tests pass
- [x] 2.2 Implement secure file persistence and verify the focused secrets tests pass
- [x] 2.3 Replace implicit selection with the explicit selection context and verify CLI construction and setup tests pass
- [x] 2.4 Update user documentation and Changelog and verify format, link, and terminology gates pass

## 3. Proof and closeout

- [x] 3.1 Run the affected Go tests with race detection and verify no controllable warning remains
- [ ] 3.2 Run the complete locked source gate and exact ETHOS proof for the finished tree
- [ ] 3.3 Archive the Change, land the exact proven commit, and retire its worktree and branch after accepted integration
