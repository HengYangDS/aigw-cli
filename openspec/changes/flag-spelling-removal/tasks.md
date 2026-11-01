## 1. Removal

- [x] 1.1 Remove the hidden `--model` of `setup` and `add` and the hidden `--target` of `install` and `uninstall`, with the replaced-spelling warning, annotation, mutual exclusion and apply-command mapping that only they use, and let the release journeys read each program's path flag from its own help instead of its role. An acceptance test and the scripts show that each removed spelling fails as an unknown flag and changes nothing.
- [x] 1.2 Record the removal in the team rollout guide and in CHANGELOG's `[Unreleased]` section, naming `--upstream-model` and `--path` as the spellings to use.
- [x] 1.3 Make every flag error name the help of the command that refused the flag, or top-level help for a command that help does not list, and record it in CHANGELOG's `[Unreleased]` section and the team rollout guide. An acceptance test shows it for unknown long and shorthand flags, a missing and an invalid value, a nested command, the root and the help command, in text and as a JSON document, and the scripts show it for each removed spelling.

## 2. Release

- [ ] 2.1 Set `VERSION` to 0.6.0 and give CHANGELOG its 0.6.0 section; `validate-changelog` passes.

## 3. Verification

- [ ] 3.1 Run the local quality graph at the release head.
- [ ] 3.2 Prove the release head with the full ETHOS proof.
- [ ] 3.3 Confirm the hosted three-platform graph and the real-client journeys from v0.5.0 at the release head.

## 4. Publication and acceptance

- [ ] 4.1 Tag v0.6.0 on GitHub, notarize its macOS archives, and publish its GitHub Release and Homebrew Cask.
- [ ] 4.2 Install 0.6.0 through Homebrew on this Mac and accept it: `aigw verify` with the real clients, then a Homebrew rollback to 0.5.0 and an upgrade back.
