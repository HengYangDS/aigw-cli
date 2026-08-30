## 1. Contract and regression

- [x] 1.1 Define the Codex credential-projection behavior and verify the OpenSpec delta targets `product-control-plane`.
- [x] 1.2 Add an admitted-client credential regression and verify the previous implementation fails for Codex.

## 2. Implementation and local verification

- [x] 2.1 Replace the Claude-only branch with the admitted-client authority and verify focused credential, Codex, synchronization, setup, and manifest tests pass.
- [x] 2.2 Run the locked source-and-governance gate and verify statement and branch coverage remain above policy floors.

## 3. Lifecycle closeout

- [ ] 3.1 Create a signed Conventional Commit and verify its signature and clean worktree.
- [ ] 3.2 Execute exact-HEAD proof, land through the local candidate train, and verify accepted `dev` contains the same Git object.
- [ ] 3.3 Verify hosted GitHub and GitLab CI for the accepted object, then publish and install the next prerelease from matching dual-Forge assets.
- [ ] 3.4 Verify the installed credential command, status, doctor, check, update, rollback, and uninstall journeys; then archive this Change and retire its Work Lane without residue.
