## 1. An Explicit Selection And A Model That AIGW Has Not Added

- [x] 1.1 Return one typed refusal for a client's model that AIGW has not added, with the restoring command as its
      next action and the adding command beside it; tests read both for Codex and for Hermes, and the terminal's
      problem shows them.
- [x] 1.2 Let `aigw use hermes` replace a Hermes selection that names an unlisted model when nothing else changed; a
      test shows the named model written and a file with another changed setting still refused.

## 2. Every Writing Command Follows The Client's Choice

- [x] 2.1 Follow a client's chosen model in the transaction and in the planner, for the clients that the transition
      writes and not for one that it selects for; tests cover a command that writes the client's files, one that
      writes none of them, an explicit selection, and a preview that changes nothing.
- [x] 2.2 Name the followed model and the returning command in the client-file row of every such result and
      preview, in text and JSON; `aigw sync` words its line from the same source and keeps `adopted_selections`.
- [ ] 2.3 Run each command that writes client files after a model was chosen in Codex and in Hermes, in an
      acceptance script: the client keeps the model, the configuration names it, and the result reports it.

## 3. Status And Check

- [ ] 3.1 Report a chosen model as the client's selection in `aigw status` and `aigw check`, marked as chosen in the
      client, in text and JSON, writing nothing; tests read the rows for Codex and Hermes.
- [ ] 3.2 Report a model that AIGW has not added with the model that the file names, the recorded selection and the
      restoring command as the next action; a script runs that command as printed and then `aigw sync`.
- [ ] 3.3 Name the model that Claude Code's own settings select in `aigw status` and `aigw check`, beside AIGW's
      selection; tests read the row and the document.

## 4. Records

- [ ] 4.1 Say in `README.md` and the documents that describe adoption what a model chosen in a client means, and
      state the change under Unreleased in `CHANGELOG.md`, the changed Fix marked as breaking.

## 5. Verification

- [ ] 5.1 Compile and vet the changed packages and their tests for Windows and Linux on the lane head.
- [ ] 5.2 Pass `mise run check` on the lane head.
- [ ] 5.3 Pass the proposal's hosted checks on both forges.
