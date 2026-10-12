# Tasks

## 1. Windows Credential-Policy Expectation

- [x] 1.1 Retain the failing Native Windows acceptance of `a278c443` (run `38235388359`, job `114764677715`) as the RED evidence.
- [x] 1.2 Require the categorized Windows refusal, zero client calls and no credential canary for Hermes in `TestExternalCredentialFailuresKeepUnknownSecretsOutOfDiagnostics`; vet and lint the darwin, linux and windows targets and rerun `internal/client` on macOS.
- [x] 1.3 Extend the refusal scenario and pass strict official OpenSpec validation.

## 2. Local Gates

- [x] 2.1 Run `mise run check` and `mise run native` once on the stable tree.

## Workflow follow-up

- Prove, land, archive and close out through ETHOS, then publish `dev` and `main` to GitLab.
- Move the GitHub proposal of pull request HengYangDS/aigw-cli#165 to the new head and publish GitHub `dev` and `main` after its required checks pass.
