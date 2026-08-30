## 1. Regression and ownership

- [x] 1.1 Add a failing readiness regression proving a Claude Route uses the
      Anthropic authentication target and headers while Codex retains its
      Responses target.
- [x] 1.2 Move shared protocol request construction into the existing
      credential-validation owner and delete the duplicate diagnostics logic;
      verify focused credential, diagnostics, and readiness tests pass.

## 2. Product verification

- [x] 2.1 Validate this OpenSpec change strictly and pass the affected package
      tests and the complete locked source gate.
- [x] 2.2 Build a candidate from the current source and verify the real installed
      configuration reports both Claude and Codex healthy without changing the
      current rc.101 installation; repeat from the exact signed commit before
      delivery.

## 3. Delivery

- [ ] 3.1 Produce exact-HEAD proof, archive the Change, and land the signed
      product through candidate and accepted truth.
- [ ] 3.2 Publish one later signed prerelease through both Forge projections,
      verify native release acceptance and assets, upgrade the installed runtime,
      and retire the Work Lane with no residue.
