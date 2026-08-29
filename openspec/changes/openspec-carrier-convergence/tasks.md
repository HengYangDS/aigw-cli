## 1. Establish the authority contract

- [x] 1.1 Validate the proposal and repository-organization delta with `openspec validate openspec-carrier-convergence --strict`.
- [x] 1.2 Confirm the accepted ETHOS runtime selects the change and admits the exact mutation paths with `ethos status` and `ethos lane prewrite`.

## 2. Remove duplicate carriers

- [x] 2.1 Delete every tracked root and per-change `commitment.toml`, then verify the staged index reports none.
- [x] 2.2 Search tracked source for a remaining Commitment-path consumer and verify no repository code or configuration depends on one.

## 3. Verify and close out

- [x] 3.1 Run the focused OpenSpec and repository-governance checks and verify they pass without a compatibility path.
- [x] 3.2 Run the affected repository gate once and verify the worktree is ready to land.
- [x] 3.3 Confirm public ETHOS lifecycle readiness from the signed change commit, then verify the closeout plan targets accepted refs and both Forge projections without adding a repository-specific path.
