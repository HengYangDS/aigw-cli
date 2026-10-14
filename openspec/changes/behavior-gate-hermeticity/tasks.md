# Tasks

## 1. Host Git policy

- [x] 1.1 Add a regression that gives the host a Git template with a failing
      pre-commit hook, injected configuration and an exclude-everything
      pattern, then builds the signed release fixture; it fails before 1.2.
- [x] 1.2 Set an empty `GIT_TEMPLATE_DIR` in the `TestMain` of every test
      package that creates repositories; the regression passes, and the
      default suite passes under that hostile host template.
- [x] 1.3 Add `tools/gitfixture` with a test that gives the host a global hook
      directory and template that reject every update; a fixture repository
      still initializes and commits, and still reads the host's other global
      settings.
- [x] 1.4 Run every test package that creates repositories through
      `gitfixture.Run`; the default suite passes under the hostile host
      template and hook directory.

## 2. Network declaration

- [x] 2.1 Move `TestNativeDependencyEvidenceRetainsFindings` behind the
      `dependency_acceptance` tag and run it as the `dependency-acceptance`
      quality-graph gate; the default suite passes with public HTTP(S)
      refused, and the gate passes with network access.

## 3. Local gates and proof

- [ ] 3.1 `mise run check` passes on the lane head.
- [ ] 3.2 ETHOS proves the exact lane head.

## 4. Hosted CI

- [ ] 4.1 GitHub CI passes on the published head, including the regressions
      on native Windows, macOS and Linux.

## Workflow follow-up

- Land through ETHOS, publish with the operator's approval, archive the Change
  and retire the lane.
