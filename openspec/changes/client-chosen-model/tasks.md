## 1. A Selection That AIGW Does Not Follow

- [x] 1.1 Return one typed refusal for a client's selection that AIGW does not follow, with the step that restores
      the recorded selection as its next action and the adding command in its text; tests read both for Codex and
      for Hermes, and a model ID that needs quoting gives prose.
- [x] 1.2 Classify Hermes's own model block once, and let `aigw use hermes` replace a selection that names an
      unlisted model while an edited address stays refused; a table test covers every state of the block.
- [x] 1.3 Report that refusal from `sync`, another writing command, a withdrawal, `status` and `check`, with one
      step; tests and a script run each, and the script runs the step as printed.
- [x] 1.4 Observe each Codex file as recorded, chosen or edited outside, so that an outside edit in any file is
      refused under its own recovery and the inspection of a followed choice is exact; tests cover a file with a
      model that was not added beside a file with an outside edit, files that agree and files that disagree.
- [x] 1.5 Present a change that was refused before anything was written in the refusal's own words, with the
      statement that nothing changed, from a command and from its preview alike, and let `aigw config rollback`
      pass such a refusal on with its Fix; tests read the three parts from `sync`, its preview, another command
      and a rollback.
- [x] 1.6 Give one owner to how Codex's files stand against the selection, read by following, by the inspection and
      by the check that runs the client, and let an outside edit in any file decide the client's next step; tests
      run `aigw check codex --depth client` after a choice, the two files in both orders, and one choice among two
      files.

## 2. Every Command Works From The Client's Choice

- [x] 2.1 Read the configuration for every command through one load that follows the clients' own choices, and bind
      the transaction's preimage to it; tests cover a command that changes the client's own settings, one that
      writes none of its files, an explicit selection and a withdrawal, and one test refuses a direct read of the
      stored configuration in the command tree.
- [x] 2.2 Report what an invocation followed once, as a section of human output and as `followed_selections`,
      for saving commands, previews, `status` and `check`; tests read each, and `aigw sync` writes no
      `adopted_selections`. Task 2.5 sets the section's place.
- [x] 2.3 Drive the built program through a choice made in Codex in an acceptance script: the reports, a writing
      command, and a model that was not added with its Fix.
- [x] 2.4 Record a followed selection together with AIGW's record of the client's file, in a command and in its
      preview; tests choose the earlier model in the client again after a command about something else, for Codex
      and for Hermes, cover a provider that waits for a Token, and the script does the same with the built program.
- [x] 2.10 Record every choice with or without its Token by a write of AIGW's record alone when the client's files
      cannot be written, and delete the waiting state it replaces (`Followed.Waiting`, the reported configuration,
      the transition's `shown` and `servesChoice`); the seventh review's eight runs pass as regression tests, and a
      record-only write leaves the client's file's identity and bytes as they were.
- [x] 2.11 Bring AIGW's record of every enabled client's files to the selection that the files show at every
      committed transition, through the one predicate that the ordinary projection uses, leave a client whose
      file is refused as stored with or without a Token, and start an explicit selection from the stored binding;
      the eighth review's twenty probes pass as regression tests with assertions, and a ninth review works from
      the state table.
- [x] 2.12 Hold one sentence for every committed command by an exhaustive test over client, files, Token, each
      file's state against its record and the command, and let one judge, the ordinary projection's admission,
      decide every client that a transition touches with a Token or without: a named client that waits for its
      Token is planned without being written, a rollback names every client whose stored binding it replaces, and
      an explicit selection takes its defaults and its report from the stored binding. The matrix fails on version
      nine and with each of the two changes switched off, and the ninth review's seventeen tests pass.
- [x] 2.13 Read a selection line of Codex's file through one TOML reader, judge `aigw client edit` against the
      stored binding, and refuse the removal of a left client's stored model with the refused file's own words.
      Done when the matrix, widened by the spelling of a pick, a checked withdrawal, a record that `follows`
      needs, the comparison of each run with its twin across the Token and the chosen provider's Token, fails on
      version ten in the tenth review's sequences and passes; the review's six finding probes pass as regression
      tests; and an eleventh review works from the widened matrix.
- [x] 2.14 Answer the eleventh review's findings two to four, and close the seven holes it found in the matrix's
      checker, in the twelfth version. A tokenless edit admits the files a client already has and refuses only
      one that drops a file; a Token that cannot be observed leaves its client waiting like an absent one, so the
      choice is recorded; a rollback over a choice restores the files it drops while its client keeps any kept
      file waiting for its Token. The checker compares the whole folder for a preview or a refusal, requires a
      command's own setting to be stored, compares each twin's whole binding except the worlds' own paths,
      requires a withdrawal to disable the client, checks a rollback against the whole restored binding, reads
      the provider whose endpoint a Codex file reaches rather than its model alone, and no longer skips a file
      that is gone. Done when the product matrix passes at this head and the eleventh review's seven hole probes
      pass inverted. A twelfth fresh review of the whole lane, on `d70552c12`, still found cases of the three
      defect families for a target named by another path and a credential store that cannot answer; they rest
      on two facts that each have two sources (design: "What stays open"). The owner decided on 2026-11-22 at
      19:43 to land the lane with them open and to fix each root cause in a Change of its own before any
      release.
- [x] 2.5 Keep one record per invocation of what was followed and report it ahead of the next step, with the
      returning command on a line of its own, the entry's protocol and ended-setting fields, no entry for the
      client that an explicit selection names, and no returning command that cannot run; tests read each, and
      `aigw use` reports whether the stored configuration changes in preview and result. `forwarding_cleared` is
      read by a test; `retries_cleared` cannot arise for a followed entry with today's clients (design: "What stays
      open").
- [x] 2.6 Name the selections that `aigw config rollback` restores, with what each replaces, in its preview and its
      result; tests read both and the client's file.
- [x] 2.7 Save a choice only in the transition that writes AIGW's record of the client's file, and leave the choice in
      the client's file untouched where that transition writes AIGW's record alone; tests choose a model of a
      provider without a Token, save, choose the earlier model again, store the Token and synchronize, and run a
      command about something else while another file of the client is missing.
- [x] 2.8 Compare the verified snapshot's configuration through the same view as the transaction; a test runs a
      check of every client at the client depth after a choice and reads the snapshot.
- [x] 2.9 Let a command that fails name no followed selection, and a report that fails name what it follows; drive
      a table of commands after a choice and without one, for Codex and for Hermes, and require each pair to end
      alike.

## 3. Claude Code

- [x] 3.1 Name the model that Claude Code's own settings select in `aigw status` and `aigw check`, in text and as
      `client_model`; tests read the row and both documents.

## 4. Records

- [x] 4.1 Say in `docs/concepts/product-concepts.md` what a model chosen in a client means, and state the changes
      under Unreleased in `CHANGELOG.md`: the three breaking changes and each document's new `schema_version`
      with its fields.
- [x] 4.2 Bring the proposal, the design, the specification deltas, the concept document and the CHANGELOG to the
      behavior of tasks 1.4, 1.5 and 2.4 to 2.6, and name in the design what stays open.

## 5. Verification

- [x] 5.1 Compile and vet the changed packages and their tests for Windows and Linux on the lane head.
- [x] 5.4 Run the exhaustive tests of a choice in a quality gate of their own, under the `selection_matrix` tag,
      without the race detector and coverage. Done when the gate passes at two cores (594 s on 2026-11-23) under a one-hour limit and
      the untagged acceptance package passes under both (351 s).
- [x] 5.2 Pass `mise run check` on the lane head (`4fb4495b2`, 95.78 %; again after each merge of `dev`: `0c43ffa10`
      and `5006895d4`, 95.78 %).
- [x] 5.3 Pass the proposal's hosted checks on both forges (`5006895d4`: GitHub 5 passed, 2 skipped; GitLab pipeline
      17380, its macOS job passing on retry 76118 after the 5-second credential bound failed under host load).
