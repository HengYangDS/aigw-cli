## 1. Specifications

- [x] 1.1 Write the deltas of `client-adapters`, `projection-format`, `secret-storage` and `client-verification`. Done
      when `openspec validate missing-program-reported-truthfully --strict` passes and the lane's archive rehearsal
      reports every merged specification within the size limit.

## 2. One rule for a client's program

- [x] 2.1 Give `internal/adapter` the one rule of D1 and make every adapter's convergence, Codex's planned targets and
      `aigw client list` use it; delete `resolveExecutable`'s fallback and `installedExecutable`. Done when no other
      function chooses between a configured and a discovered program and the package tests of `internal/adapter/...`
      and `internal/cli/client` pass.
- [x] 2.2 Write every test's fake client executable through one shared, ETXTBSY-safe helper,
      `internal/platform/processtest`. Done when no test package keeps a copy of its own.

## 3. A missing program stops its client

- [x] 3.1 Leave an enabled client whose configured program is missing out of every projection that synchronization
      plans, and refuse it in the adapter registry's plan (D2). Done when the stale-loop regression (the committee's
      experiment 1, at the adapter layer with a fake program directory) and the package tests of
      `internal/app/synchronization` pass. `TestAMissingCodexProgramEndsTheStaleLoop` failed on `9421e3d6e` (the second
      sync succeeded and rewrote `config.toml`) and passes here.
- [x] 3.2 Make `aigw sync` synchronize the other clients, then name each stopped client with its repair and exit
      non-zero, in a preview too (D2). Done when the package tests of `internal/cli/sync` pass.
- [x] 3.3 Give a missing program the repair `aigw client edit <client> --executable <path>` in the check of Codex,
      Claude Code, Claude Desktop and Hermes (D2). Done when a test of each client's check passes.
- [x] 3.4 Stop a Codex whose configured executable runs and can neither produce nor reuse the model table that AIGW
      owns, keeping its files, with the same problem and fix in sync and check (D2). Done when
      `TestACodexThatCannotProduceItsTableStops` passes, and `TestACodexThatNeverProducedATableIsNoLoop` pins that a Codex
      that never produced a table is no loop.
- [x] 3.5 Withdraw a file that the change drops from a stopped client (D2). Done when
      `TestADroppedFileIsWithdrawnWhileCodexIsStopped` passes.
- [x] 3.6 Report each stop through one point in every command that commits a projection, with what it wrote, exiting
      non-zero, and list the stopped clients in a sync preview (D2). Done when
      `TestUseAndRollbackReportAStoppedCodex`, `TestProviderCommandsReportAStoppedCodex`, `TestSetupReportsAStoppedCodex`,
      `TestASyncedCodexWhoseExecutableGoesStopsUntilAnotherIsNamed` and the `internal/cli/sync` tests pass.
- [x] 3.7 Refuse at input an `--executable` that is not a regular file with an execute bit, by stat alone (D2). Done when
      `TestAnExecutableThatCannotRunIsRefused` and `sync.txtar` pass.
- [x] 3.8 Count a client as installed in setup through the one rule of D1. Done when
      `TestManifestSetupAvailabilityTakesTheConfiguredProgramFirst` passes.
- [x] 3.9 Refuse a commit or a preview that has no record of the clients that it stops (D2). Done when the package tests
      of `internal/app/synchronization` and `internal/cli/...` pass with the record given by every test context.

## 4. The model table's producer

- [x] 4.1 Record the producing program's path and its package manifest's version in Codex's record beside its digest
      (D3). Done when a Codex projection test reads them back from the record.
- [x] 4.2 Compare the configured program with the record in `aigw check --depth local` by path, digest and manifest
      version against the recorded manifest, with the one manifest layout read on an installation declared as data,
      running no program while they agree (D3). Done when tests of a program changed in place, another path, a
      manifest version change, a removed manifest, a program without a manifest and a record without a producer pass.

## 5. Reasons

- [x] 5.1 Name a stale table, a table from another program and a missing program in `aigw check`, and keep "does not
      match the selected model" for a real mismatch (D4). Done when the check tests of each reason pass.

## 6. Documents

- [x] 6.1 State in the terminal-experience document that a dry run runs read-only probes of the client program in an
      isolated temporary directory (D5), and in the team rollout guide that AIGW keeps a client's executable and what
      to run when it goes. Done when the prose and Markdown gates pass.
- [x] 6.2 Add the changed behavior and output under `## Unreleased` in `CHANGELOG.md`. Done when
      `go run ./tools/release validate-changelog` passes.

## 7. Local gates

- [ ] 7.1 Pass `mise run check` on the lane head, in the host queue.
- [ ] 7.2 Pass `mise run native` on the macOS host on the lane head, in the host queue.

## 8. Review

- [x] 8.1 A fresh review of the whole change by a reviewer who has not seen it. Done when its findings are answered in
      code or in the design. Twelve findings (`build/evidence/lane-20261125-program-truth/independent-review.md` in the
      main checkout), answered by the coordinator's rulings of 2026-11-26 in D2, D3 and D4.

## 9. Exact-HEAD proof

- [ ] 9.1 Pass the ETHOS full proof of the lane head.

## 10. Hosted CI

- [ ] 10.1 Pass the proposal's GitHub checks `Native macOS acceptance`, `Native Linux acceptance`,
      `Linux Secret Service` and `Quality and governance`.
- [ ] 10.2 Pass the proposal's GitLab pipeline on the macOS and Linux runners.

## 11. Publication

- [ ] 11.1 Publish the proven lane head as the lane's proposal branch on both forges. Done when each forge reads back
      the exact head.
