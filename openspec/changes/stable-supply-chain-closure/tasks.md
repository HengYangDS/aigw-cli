## 1. Stable Inventory

- [x] 1.1 Audit every direct runtime, Go module, npm package, Mise tool, CI
      Action, container image, quality tool, security tool, and release tool against
      its authoritative stable release source.
- [x] 1.2 Classify each newer version as compatible, incompatible with an exact
      reason, or not directly owned; do not add direct pins for transitive-only
      updates.

## 2. Deterministic Refresh

- [x] 2.1 Update direct declarations in their existing owners and delete every
      superseded version literal.
- [x] 2.2 Regenerate Go, npm, Mise, OCI, Action, and CUE-derived projections with
      native tools.
- [x] 2.3 Run every resolver twice and prove the second run is byte-clean.

## 3. Acceptance

- [x] 3.1 Run strict OpenSpec validation, bootstrap, dependency-policy checks,
      focused dependency tests, and the complete source gate.
- [x] 3.2 Run native acceptance and deterministic release construction without
      replacing the installed product.
- [x] 3.3 Verify no warning, vulnerability, unsupported-platform regression,
      duplicate owner, compatibility residue, or unaccounted version literal
      remains.

## 4. Integration

- [x] 4.1 Commit the clean signed candidate, obtain exact-HEAD ETHOS proof, and
      integrate through the governed candidate and accepted branches.
