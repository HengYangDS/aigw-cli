## 1. Fix

- [x] 1.1 Read accessible Token prompts with terminal echo disabled and decide
      accessibility once from the CLI environment; a regression test proves a
      terminal Token never reaches the prompt output or the plain line reader.

## 2. Release

- [x] 2.1 Set `VERSION` to 0.6.1 and give CHANGELOG its 0.6.1 section;
      `validate-changelog` passes.

## 3. Verification

- [x] 3.1 Run the local quality graph at the release head.
- [x] 3.2 Prove the release head with the full ETHOS proof.
- [x] 3.3 Confirm the hosted three-platform graph and the real-client journeys
      from v0.6.0 at the release head.

## 4. Publication and acceptance

- [x] 4.1 Tag v0.6.1 on GitHub, notarize its macOS archives, and publish its
      GitHub Release and Homebrew Cask.
- [x] 4.2 Install 0.6.1 through Homebrew on this Mac and accept it: a Token
      typed at an accessible prompt in a terminal is not echoed, `aigw verify`
      passes with the real clients, and a Homebrew rollback to 0.6.0 and an
      upgrade back succeed.
