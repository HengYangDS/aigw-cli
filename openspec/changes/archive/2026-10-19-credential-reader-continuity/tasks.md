# Tasks

## 1. Capability Component

- [x] 1.1 Make Store tests expect a saved `tool_search` only in the capability
      component, the full program to still declare it, the canonical location
      to move on the next write, an unmatched entry to be dropped, and a
      damaged component to fail full reads but not credential lookup; they
      fail before the component exists.
- [x] 1.2 Make recovery tests expect the component through compensation,
      backup convergence, backup reads and verified checkpoints, with no
      `tool_search` in the checkpoint; they fail before the component exists.
- [x] 1.3 Store the component and include it in snapshots, recovery and
      equality; the tests pass.

## 2. Credential Diagnostics

- [x] 2.1 Make credential helper tests expect the reader version and the
      failed stage, plus the operation and error without the path for a read
      failure; they fail before the diagnostic exists.
- [x] 2.2 Report them in the configuration problem; the tests pass.

## 3. Documentation

- [x] 3.1 Describe the component in `docs/guides/team-rollout.md` and the fix
      in `CHANGELOG.md`.

## 4. Verification

- [x] 4.1 `mise run check` passes on the change set.
- [x] 4.2 Every retained published reader returns the fixture Token from a
      configuration the candidate saved with `tool_search` declared, and from
      a 0.4.5 configuration after the candidate's `aigw sync`
      (`build/evidence/credential-reader-continuity/acceptance-candidate-1.txt`).
