## 1. A Selection That AIGW Does Not Follow

- [x] 1.1 Return one typed refusal for a client's selection that AIGW does not follow, with the step that restores
      the recorded selection as its next action and the adding command in its text; tests read both for Codex and
      for Hermes, and a model ID that needs quoting gives prose.
- [x] 1.2 Classify Hermes's own model block once, and let `aigw use hermes` replace a selection that names an
      unlisted model while an edited address stays refused; a table test covers every state of the block.
- [x] 1.3 Report that refusal from `sync`, another writing command, a withdrawal, `status` and `check`, with one
      step; tests and a script run each, and the script runs the step as printed.

## 2. Every Command Works From The Client's Choice

- [x] 2.1 Read the configuration for every command through one load that follows the clients' own choices, and bind
      the transaction's preimage to it; tests cover a command that changes the client's own settings, one that
      writes none of its files, an explicit selection and a withdrawal, and one test refuses a direct read of the
      stored configuration in the command tree.
- [x] 2.2 Report what an invocation followed once, as the last section of human output and as
      `followed_selections`, for saving commands, previews, `status` and `check`; tests read each, and `aigw sync`
      writes no `adopted_selections`.
- [x] 2.3 Drive the built program through a choice made in Codex in an acceptance script: the reports, a writing
      command, and a model that was not added with its Fix.

## 3. Claude Code

- [x] 3.1 Name the model that Claude Code's own settings select in `aigw status` and `aigw check`, in text and as
      `client_model`; tests read the row and both documents.

## 4. Records

- [x] 4.1 Say in `docs/concepts/product-concepts.md` what a model chosen in a client means, and state the changes
      under Unreleased in `CHANGELOG.md`: the two breaking changes and each document's new `schema_version` with
      its fields.

## 5. Verification

- [ ] 5.1 Compile and vet the changed packages and their tests for Windows and Linux on the lane head.
- [ ] 5.2 Pass `mise run check` on the lane head.
- [ ] 5.3 Pass the proposal's hosted checks on both forges.
