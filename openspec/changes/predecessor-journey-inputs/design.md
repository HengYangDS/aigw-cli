## Context

The native client journey (`TestNativeClientJourney`, `tools/release/clients_test.go`) installs a baseline program,
applies the shipped team file with it, enables a real client and then walks four lifecycle steps: baseline,
candidate, rollback and re-upgrade. After each step the candidate judges the client with
`check <client> --depth client`; since 2026-10-11 every step is judged by the candidate, the current verifier. In
default runs the baseline is a source build at version 0.0.0; with `AIGW_ACCEPTANCE_BASELINE` it is a published
predecessor, which for 0.11.0 is 0.10.0.

The proposal names the two inputs that do not suit 0.10.0. The facts below are readings, not measurements:

- `prepareNativeClient` is the one function that exports the shipped team file for a journey; seven calls use it.
  Two journeys apply its output with the baseline: `plan.run` and `runCodexConnectionRetries`. The others start on
  the candidate.
- v0.10.0's record of Codex's model table has no `catalog_client_path` (`internal/adapter/codex/config.go` at the
  tag), and the candidate's `producerProblem` reports such a record until a sync records the executable.
- v0.10.0 wrote a Hermes provider's `models` as a sequence; the candidate writes a mapping, and its readiness compares
  the managed bytes, so a file that 0.10.0 wrote reads as differing until a sync.
- v0.10.0's `self rollback` restores the program and not the configuration (`internal/cli/self/rollback.go` at the
  tag): it runs a configuration-read check and writes no client file. After the rollback step the client files are
  still the ones the candidate wrote.

## Goals / Non-Goals

Goal: the 0.11.0 candidate acceptance reaches its first real judgement with 0.10.0 as the baseline, without
weakening what the journeys check. Non-goals: any change of the product or its release notes, a sync that repairs
client files unseen, and native coverage of a baseline other than a published predecessor or the source build.

## Decisions

1. **The baseline's team file loses its windows only when the baseline is a published predecessor.** The plan
   records once whether its baseline is published (`publishedBaseline`). `prepareNativeClient` writes the shipped
   file as before; `baselineManifest` returns it for a source build, and for a published predecessor writes a copy
   without any model's `context_window`, which the two baseline journeys apply.
   Alternatives: clearing inside `prepareNativeClient` for every journey would remove the declared windows from the
   candidate's own native journeys, the only native evidence for them; moving `setup` to the candidate would leave a
   configuration that 0.10.0's `self rollback` refuses at the rollback step. Removal: when the oldest admissible
   baseline reads `context_window` (0.11.0 or later), the helper, its two calls and the re-application of decision 3
   go.
2. **Only the baseline step synchronizes before the candidate judges it.** It is the one step whose client files
   another program wrote; the candidate step already synchronizes before the update (`preprojectReplacement` until
   this Change), and the rollback writes no client file. The baseline step reuses that function, renamed
   `synchronizeWithCandidate` for both uses, and a field of the step type (`synchronize`) marks the one step that
   needs it.
   Alternatives: judging the baseline step with the baseline program would reverse the decision of 2026-10-11;
   synchronizing before every step where the baseline is installed would also repair, unseen, any client file that a
   rollback disturbed.
3. **The windows return after the lifecycle.** With a published baseline, no native journey of Hermes would see a
   declared window, and `accept-native --clients` always has one. After the re-upgrade step, the installed candidate
   applies the shipped file (`requireDeclaredWindows`) and judges each client whose specification runs models at
   their windows; Hermes's file must give each listed model its configured window, and at least one must be declared.
   It comes after the last rollback because 0.10.0's `self rollback` refuses a configuration that declares a window,
   and before forwarding, editing and removal, which then run with the windows. The check of Hermes's file compares
   with the configuration, so the published-predecessor journey, whose file declares none, still requires the
   default for each model. Alternative: correcting the documents and leaving the gap would release declared windows
   with no automated native evidence from Hermes.

## Risks / Trade-offs

- The facts above are read, not run. The candidate acceptance of 0.11.0 measures them:
  `accept-native --artifacts <dist> --candidate --clients --baseline-tag v0.10.0`. A further failure there goes into
  a Change of its own.
- In every run with the four lifecycle steps the added sync repeats what the candidate step's sync before the update
  does, since only the baseline step's check runs between them; the second is kept to check that the candidate's
  sync leaves the configuration and credentials as they were before it replaces the program. It costs one more sync
  per client.
- Decision 3 adds one `setup` and one client turn for Codex and for Hermes in each published-baseline run.
