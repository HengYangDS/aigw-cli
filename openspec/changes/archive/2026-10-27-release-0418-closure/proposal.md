## Why

Release 0.4.18 (Change `release-0418`) left three kinds of work open.

- **Maintenance branch absorption.** The user ruled that
  `proposal/maintenance-client-qualification` (tip `f3c7f87f`, tag `v0.3.5`) is
  deleted only after every obligation it carries is absorbed into `dev`. A
  recheck of the audit
  (`build/evidence/maintenance-absorption-audit-20261025/audit.md`, item 1)
  against `dev` `35387822` finds three still open:
  - the branch test `TestCodexForwardingPreviewRejectsOwnedTargetConflict`: a
    forwarding preview refuses a projected Codex target whose managed settings
    were edited outside AIGW, reads no credential and changes no file;
  - the branch test `TestCodexForwardingPreviewIncludesNewlyDiscoveredTarget`:
    a forwarding preview plans a newly discovered Codex target without creating
    it, and the selection then writes that same file;
  - the branch's `projection-format` delta: forwarding shares the guarded
    snapshot, backup, checkpoint and compensation boundary, a retained reader
    still decodes forwarded configuration, and a stale forwarding commit is
    refused. `dev` implements and tests these; its specification does not say
    so.
- **Real-client journey on a current baseline.** GitHub Verify run
  40511720850 (published baseline v0.4.17, candidate v0.4.18) passed every
  Claude Code, Codex and Hermes phase on Linux, macOS and Windows, but
  `codex-connection-retries` failed on all three: "retained reader did not
  refuse policy at its native boundary". The journey assumes that the baseline
  cannot read a retry policy. Retry policies shipped in 0.4.17, so a v0.4.17
  baseline reads the declaration and rollback is correctly admitted.
- **Windows benchmark preflight.** 0.4.18 made a Unix preflight diagnostic mark
  its workload instead of stopping it, but the Windows preflight still turns a
  product diagnostic into an error and retains its streams as `.execution`
  rather than `.preflight`, so `Measure` never reads the verdict and the shared
  caller stops before the remaining workloads (Codex source observation
  `5bf0b019`).

## What Changes

- Port the two preview tests to `dev`'s selection test harness.
- State the forwarding boundary in `projection-format`, bound to the tests that
  already enforce it.
- Make the retry journey expect the boundary that the baseline actually has: a
  baseline before 0.4.17 refuses rollback without changing either program; a
  baseline from 0.4.17 on takes the rollback with the policy selected, and the
  candidate is restored through the same rollback.
- Make the Windows preflight retain `.preflight` streams, its preparation's
  included, and leave the diagnostic verdict to `Measure`, as on Unix.

Once this Change is delivered, the maintenance branch ref is retired; its tags
`v0.3.5` and `native-inputs-f3c7f87f…` stay.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `projection-format`: forwarding participates in the projection conflict and
  compensation boundary.

## Impact

Tests, release acceptance tooling and specification only; no product behavior,
configuration format or release changes. The branch ref is retired after
delivery; its history stays reachable through `v0.3.5`.
