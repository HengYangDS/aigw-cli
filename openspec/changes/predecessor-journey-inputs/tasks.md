## 1. The predecessor journey's inputs

- [x] 1.1 Give a published baseline the shipped team file without its models' context windows, in the per-client
      journey and the Codex connection-retries journey. Done when `go vet -tags client_acceptance ./tools/release`
      passes and the candidate's journeys still prepare the unchanged team file.
- [x] 1.2 Run the candidate's sync with its configuration and credential checks before the candidate judges the
      baseline step. Done when `go vet -tags client_acceptance ./tools/release` passes and the baseline step is the
      only step that does so.
- [x] 1.3 After the re-upgrade step, apply the shipped file with the installed candidate, judge Hermes with it, and
      check Hermes's windows against the configuration per entry and protocol; stop a Hermes journey at its start when
      the shipped file declares no window for a model that Hermes would list. Done when
      `go vet -tags client_acceptance ./tools/release` passes and both checks fail when no listed model declares a
      window.

## 2. Verification

- [x] 2.1 A fresh review of this Change by a reviewer who has not seen it. Done when its findings are answered in code
      or in this design.
- [ ] 2.2 Pass `mise run check` on the lane head.
- [ ] 2.3 Pass the proposal's hosted checks on both Forges.
