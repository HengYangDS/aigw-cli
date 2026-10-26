## 1. File Names In Doctor

- [x] 1.1 Make the doctor tests expect each Codex target check under the home
      directory to name its file as `~/...` in its label and JSON `target`, and
      each target outside it to stay unnamed with an outside-home label. Before
      the change the new label and `target` assertions failed; the absolute-path
      variant failed `TestDoctorJSONDoesNotExposeMissingCodexTarget`.
- [x] 1.2 Carry the inspected file on adapter checks and publish only its
      home-relative form; the doctor, client and CLI acceptance tests pass.

## 2. Release

- [x] 2.1 Prepare 0.4.17: `VERSION` and a dated `CHANGELOG.md` section that
      records every user-relevant change since 0.4.16; `validate-changelog`
      passes.

## 3. Verification

- [ ] 3.1 `mise run check` passes on the lane head.
- [ ] 3.2 The full proof of the lane head passes.
