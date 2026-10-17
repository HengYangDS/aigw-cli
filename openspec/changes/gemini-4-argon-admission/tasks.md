# Tasks

## 1. Replace the Gemini Model

- [x] 1.1 Record the October 16 primary-source, catalogue and native Hermes
      evidence for `gemini-4-argon` under the verification owner.
- [x] 1.2 Make the team manifest tests expect `gemini-4-argon`, its three
      Routes and no Gemini 3.8 Flash Route; they fail on the current manifest.
- [x] 1.3 Replace the Gemini Model and Routes in `manifests/team.toml`; the
      team manifest tests pass.
- [x] 1.4 Record the admission in `docs/research/provider-model-qualification.md`
      within its length limit, and add the change to `CHANGELOG.md`.

## 2. Verification and release

- [x] 2.1 `mise run check` passes, and the lane head is proved.
- [x] 2.2 Release the change with the next AIGW release through the existing
      release workflow (`v0.4.3`).
