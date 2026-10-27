## 1. Test Hermeticity

- [x] 1.1 Make the Mise configuration test write Mise state inside its
      workspace. Its new private-state assertion fails before the change and
      passes after it.

## 2. Doctor

- [x] 2.1 Move doctor's target labelling into its own file with no behavior
      change; the doctor tests pass.
- [ ] 2.2 Label a target on another volume as outside the home directory. The
      Windows test for another drive, a UNC share, a letter-case difference and
      a relative HOME passes in hosted Verify on Windows; it builds and vets for
      Windows locally.

## 3. Forwarding

- [x] 3.1 Record the forwarded upstream identity and protocol and refuse edits
      that change them until `--direct`. The drift tests fail against the
      reverted guard and pass with it; the same-upstream, direct-selection and
      checkpoint-shape tests pin the behavior that the change keeps.
- [x] 3.2 Refuse the same drift through the public commands. The CLI acceptance
      test for `account edit` under forwarding fails before the change and
      passes after it, with client files byte-identical.
- [x] 3.3 Refuse `--direct` combined with an empty `--forwarding-endpoint` with
      the direct-connection message. Its test fails before the change and
      passes after it.
- [x] 3.4 Port the branch's forwarding store tests; each fails against a locally
      reverted guard and passes against the shipped one.
- [x] 3.5 Document forwarding in the README and the CHANGELOG.

## 4. Codex Conflict Recovery

- [x] 4.1 Give a refused outside edit to managed Codex settings one next action
      across errors, `check`, `status` and `doctor`. The acceptance test for
      outside retry-budget edits fails before the change and passes after it,
      and undoing the edit lets `sync` succeed.
- [x] 4.2 Record the fix in the CHANGELOG.

## 5. Release Tooling

- [x] 5.1 Fail the dependency gate on an OSV exit 1 without a normalized
      advisory. Its test fails before the change and passes after it.
- [x] 5.2 Disable Hermes lazy installs in the release journey. Its test fails
      before the change and passes after it.
- [ ] 5.3 Judge benchmark diagnostics from an untimed preflight and keep
      Hyperfine's warnings as evidence; rewrite the governance rule. The new
      tests fail before the change and pass after it.
- [ ] 5.4 Port the branch's release-input, carrier and provenance regression
      tests; each fails against a locally reverted guard and passes against
      the shipped one.

## 6. CI

- [x] 6.1 Ignore non-`v` tags in release selection and let a manual tag-ref
      dispatch honor `native_platform`. The tests fail before the change and
      pass after it.
- [ ] 6.2 Index the artifact tool closure by peer and regenerate both Forge
      projections. The tool-closure tests fail before the change and pass
      after it, and `tools/ci project --check` passes.

## 7. Team Catalogue

- [ ] 7.1 Qualify each listed Route of GPT-6.1 Terra, DeepSeek V4.1 Flash 1017,
      Kimi K3.1 Pro, Qwen3.8 Max 0902, Doubao Seed 2.1 Pro 260915, MiniMax M3.1
      Flash, Muse Wonder, Mistral Large 4, Command A+ and Solar Pro 4 with
      bounded inference and the native Hermes file-tool and recall journey, and
      record the results.
- [ ] 7.2 Update the team manifest, the qualification record and the rollout
      guide for the qualified candidates; manifest tests pass.

## 8. Release

- [ ] 8.1 Set `VERSION` to 0.4.18 and add its CHANGELOG section;
      `validate-changelog` passes.

## 9. Verification

- [ ] 9.1 `mise run check` passes on the lane head.
- [ ] 9.2 The full proof of the lane head passes.
- [ ] 9.3 Hosted Verify passes on Linux, macOS and Windows for the proposal.
