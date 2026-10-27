## 1. Preview Tests

- [x] 1.1 Port the branch test for a forwarding preview of an edited owned Codex
      target. It refuses with the typed Codex conflict, reads no credential and
      leaves configuration, recovery inputs and target files byte-identical; it
      fails when the managed-block hash check is reverted locally and passes
      against the shipped code.
- [x] 1.2 Port the branch test for a forwarding preview of a newly discovered
      Codex target. The preview plans it as `initial-project` without creating
      it, reads no credential, and the selection then writes that same file; it
      fails when the preview stops planning discovered Codex homes and passes
      against the shipped code.

## 2. Specification

- [x] 2.1 State the forwarding boundary in `projection-format`; strict OpenSpec
      validation passes and every cited test exists on the lane head.

## 3. Retry Journey

- [ ] 3.1 Make the retry journey expect the baseline's own rollback boundary:
      refusal with both programs unchanged before 0.4.17, and from 0.4.17 on a
      rollback that keeps the policy, followed by a rollback that restores the
      candidate. The journey builds under `client_acceptance` and vets.
- [ ] 3.2 GitHub Verify with `native_clients=true`, published baseline v0.4.17
      and candidate v0.4.18 passes on Linux, macOS and Windows for the
      proposal, `codex-connection-retries` included.

## 4. Windows Preflight

- [ ] 4.1 Make the Windows preflight retain its preparation and observed streams
      as `.preflight` streams and leave the diagnostic verdict to `Measure`, as
      on Unix. The Windows tests for retained streams and for a marked,
      unstopped workload are updated, and the package builds and vets for
      windows/amd64 and windows/arm64.
- [ ] 4.2 GitHub Verify with `full_quality=true` passes on Windows for the
      proposal, the native attribution test included.

## 5. Verification

- [ ] 5.1 `mise run check` passes on the lane head.
- [ ] 5.2 The full proof of the lane head passes.
- [ ] 5.3 Hosted Verify passes on Linux, macOS and Windows for the proposal.
