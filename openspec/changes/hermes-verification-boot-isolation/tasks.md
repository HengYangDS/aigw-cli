# Tasks

## 1. Boot-Record Isolation

- [x] 1.1 Add adapter, configuration, CLI acceptance and native boot regressions that require borrowed dependencies, private boot records, the original selection lock, disabled launcher publication and Windows refusal; run them against accepted `f12650c8` and retain the expected RED results.
- [x] 1.2 Port `CreateVerificationHome` and `borrowVerificationInstall` from `46f451c4`, select the Windows refusal at build time, route Hermes verification through them, and remove the whole-directory sharing; rerun the focused packages and the `native_hermes` suite to GREEN, and vet and lint the darwin, linux and windows targets.
- [x] 1.3 Record the user-visible change in the Unreleased Changelog and pass strict official OpenSpec validation for this Change.

## 2. Local Gates and Host Check

- [x] 2.1 Run `mise run check` and `mise run native` once on the stable tree without warnings, failed subtests, or test-owned residue.
- [x] 2.2 Run one source-built `aigw verify --for hermes` against the installed Hermes, then repeat its layout, policy and arguments with the operator's signed credential reader; verify the version probe, the `AIGW_OK` response, unchanged boot records, dependency selection, launchers and AIGW data, and that no disposable home remains.

## Workflow follow-up

- Obtain exact-HEAD ETHOS proof, land on local `candidate/dev`, archive through `ethos lane archive-change`, and close out local `dev`/`main`; ETHOS Attestations own those results.
- Tell the terminal lane owner how this port meets `46f451c4` at merge time.
- After a signed candidate is installed through the release workflow, run `aigw verify --for hermes` against the operator's Hermes.
- Qualify the Windows refusal on a native Windows runner before publication.
- Publish only with operator approval.
