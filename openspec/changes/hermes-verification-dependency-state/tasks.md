# Tasks

## 1. Hermes Verification Dependency State

- [x] 1.1 Add a focused adapter regression that requires the disposable home to reuse the configured home's dependency environment and managed tools with on-demand installation refused; run it against the unchanged adapter and retain the expected RED result.
- [x] 1.2 Share the installed Hermes stores into the disposable home, refuse on-demand installation for every probe, and fail closed when a store cannot be shared; rerun the focused regression and every Hermes test in `internal/client` and `internal/hermes/configuration` to GREEN.
- [x] 1.3 Record the user-visible fix in the Unreleased Changelog and pass strict official OpenSpec validation for this Change.

## 2. Local Gates and Real-Client Journey

- [x] 2.1 Run `mise run check` and `mise run native` once on the stable tree; retain their results without warnings, failed subtests, or test-owned residue.
- [x] 2.2 Run the installed AIGW and a source-built candidate against the operator's installed Hermes, then repeat the candidate's disposable layout, policy, and arguments with the operator's signed credential reader; verify the version probe, the `AIGW_OK` response, and unchanged Hermes configuration, installation state, launchers, and AIGW configuration.

## 3. Upstream Report

- [x] 3.1 Report the Hermes per-task home launcher republication defect to the official Hermes repository with its reproduction, and record the verified report URL and state in `design.md`.

## 4. Governed Integration

- [ ] 4.1 Create the signed commit, obtain exact-HEAD ETHOS proof, and land it on local `candidate/dev`; verify the candidate head and keep hosted CI, publication, installation, and lane retirement as separate obligations.

## Workflow follow-up

- After a signed candidate is installed through the release workflow, run `aigw verify --for hermes` against the operator's Hermes and record the reported version and digest.
- Publish only with operator approval, then observe each peer's CI and retire the Work Lane through ETHOS.
