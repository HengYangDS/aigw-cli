# Tasks

## 1. Hermes Verification Contract

- [x] 1.1 Extend the existing Hermes adapter test to reject a verification home that permits passive update checks or exposes a raw version-probe failure; run the focused test and retain its expected RED result.
- [x] 1.2 Use Hermes' native update-check opt-out only in the disposable verification home and classify timeout, cancellation, and other version-probe failures without raw output; rerun the focused adapter tests to GREEN.
- [x] 1.3 Run the installed Hermes CLI against one disposable source-built test candidate and a local authenticated endpoint through the focused real-client journey; verify version, selected-model response, Account rename, finalization, and user-file preservation without treating the test bytes as a release.

## 2. New Release Identity and Local Acceptance

- [x] 2.1 Select the next patch version without moving or reusing the unpublished local `v0.3.2` tag; fold its user changes into 0.3.3 and validate the Changelog with both local and clean remote-equivalent tag refs.
- [x] 2.2 Run the complete source and native gates plus strict OpenSpec validation on the repaired tree; verify no warnings, failed subtests, or test-owned residue before exact-HEAD proof.

## 3. Governed Integration and Distribution

- [ ] 3.1 Obtain exact-HEAD ETHOS proof, land the signed source through its authorized lane and candidate path, and verify clean local `main`/`dev` plus each peer's required review and branch CI at that SHA.
- [ ] 3.2 Build the final Developer ID distribution once, submit its exact two-architecture ZIP for Apple notarization, and require `Accepted` plus the product verifier's ZIP, executable, certificate, and submission-ID match; run the full Claude/Codex/Hermes real-client journey against those exact bytes and the verified 0.3.1 predecessor.
- [ ] 3.3 On a qualified quiet host, run the existing published-predecessor performance acceptance against the same candidate bytes; retain raw samples and budget verdict rather than inferring performance from functional CI.
