## Context

A client's model can differ from AIGW's record in these ways, which the owner and I listed on 2026-11-20:

| How the two came to differ                                      | What the person wants | Who wins          |
| --------------------------------------------------------------- | --------------------- | ----------------- |
| The person chose another added model in Codex or Hermes         | that model            | the client        |
| The person chose a model in Claude Code                         | that model            | the client        |
| The person chose a model in a Claude Desktop conversation       | that model, there     | the client        |
| The person ran `aigw use`                                       | the named model       | AIGW              |
| The client's file names a model that AIGW has not added         | not known             | neither: reported |
| After `aigw use`, a running client wrote its earlier model back | the named model       | not told apart    |

The last row cannot be told from the first by reading the file, so it is followed as the first is; the owner accepted
that cost on 2026-11-20.

Three values decide each case for Codex and Hermes: the model that the client's file selects, the model that AIGW
last wrote there (its record of the file), and the selection in the stored configuration.

| The file against the record | The configuration against the record | What happened        | What AIGW does                   |
| --------------------------- | ------------------------------------ | -------------------- | -------------------------------- |
| the same                    | the same                             | nothing              | nothing                          |
| another added model         | the same                             | the client chose     | follows the file                 |
| the same                    | another model                        | AIGW's selection won | writes the configuration's       |
| a model that was not added  | the same                             | not known            | refuses, with the restoring step |

The table holds only while the record moves with every followed choice. A choice that is saved while the record
stays behind turns the second row into the third the next time the person chooses the earlier model.

Release 0.8.3 follows the second row in `aigw sync` alone. Every command that writes client files ends in one
transaction (`Synchronizer.commit`) and every preview in one planner; each adapter's plan refuses a file whose model
equals neither AIGW's record nor the new configuration's model. So a command other than sync refuses until sync has
run.

Two earlier versions of this Change were reviewed by a fresh reader who ran them. The first followed the choice
inside the transaction, which then saved a configuration that the command never saw: `aigw client edit hermes --via`
lost its own edit, and results printed the unfollowed configuration. The second read a working configuration at
load, as below, and still had these faults, found in about 45 runs: a recorded choice left AIGW's record of the
client's file behind, so that the earlier model, chosen again, read as no choice and `aigw sync` wrote the first
choice over it; `aigw config rollback` wrote over a choice without a word and hid a refusal behind "did not
complete"; two Codex files, one with a model that was not added and one with an outside edit, were given a Fix that
the edit refused; Codex's inspection reported a file as configured over pending drift; the report stood after the
next step, listed an explicit selection as followed and named a returning command that could not run. The third
version recorded a choice with AIGW's record of the client's file, and a third review found what it had left: the
check that runs the client, and the verified snapshot that it writes, still compared against the stored
configuration and the strict projection; a failed command named a followed selection that it had not saved; and a
choice was still saved without its record while its provider had no Token, so that a person who then chose the
earlier model again lost that choice once the Token was stored. The decisions below are the fourth version, and its
failure state was removed, not handled further, by the eighth (see "A choice is recorded with or without its Token").

Measured on 2026-11-20 with the installed 0.8.3 in an isolated home, the clients' writes imitated by changing the one
value in each file: the behaviors that the proposal lists. Not measured: what a real Codex or Hermes writes when a
model is chosen in it; the adoption of 2026-10-16 for Hermes followed a file that the owner's own Hermes had written,
and the one for Codex of 2026-10-19 rests on an imitation. The next Change in this lane measures both.

## Goals / Non-Goals

**Goals:**

- A model chosen in Codex or Hermes is kept by every AIGW command and reported as the client's selection. The two
  commands that replace it, `aigw use` for that client and `aigw config rollback`, are asked to, and name what they
  replace.
- A value that AIGW cannot follow has one step that restores a working state, unless an outside edit stands beside
  it, whose own recovery comes first.
- One owner decides what a client's own selection means, one owner reads the configuration for commands, one owner
  keeps what an invocation followed, and one type says that a refusal changed nothing.

**Non-Goals:**

- Recording a model that Claude Code or Claude Desktop chose: AIGW cannot map an alias such as `opus[1m]` to a model
  of a provider, and a Desktop conversation's model is not in a file that AIGW owns.
- Telling a stale write of a running client from a choice.
- Any sentence about what a running client does after `aigw use`; it waits for the measurement.

## Decisions

### Commands read a working configuration that follows the clients

- **Rests on:** every command loaded the stored configuration itself, at 26 places, and built its change and its
  result from it. Tests run a command that changes the client's own settings, one that names nothing of the client,
  an explicit selection and a withdrawal after a choice, and read the client's file, the saved configuration and the
  result; one test refuses a direct read of the stored configuration anywhere in the command tree.
- **Considered:** follow inside the transaction, after the command built its change; follow inside each adapter's
  plan; follow where the configuration is read.
- **Chosen because:** only a follow before the command's own change lets that change, its validation and its result
  see one configuration. The transaction cannot be the place, as the first version showed, and an adapter's plan
  cannot change the configuration.
- **Applies to:** `Dependencies.WorkingConfiguration` and `LoadConfiguration` in `internal/app`, and
  `Synchronizer.Working` in `internal/app/synchronization`. A choice that AIGW does not follow, a file that it cannot
  read, or a followed selection that would not validate leaves that client as stored, so a command about something
  else is not stopped by a client's file.

### One view of the stored configuration for every comparison with it

- **Rests on:** `commit` refuses when the stored configuration no longer matches what the command read, and the
  check that writes a verified snapshot refuses in the same way; a command now reads the working configuration,
  which differs from the stored one after a choice. The third review ran a check of every client after a choice:
  the snapshot was refused as "configuration changed during the check", with the same check as its Fix. Tests run
  that check and compare through the view.
- **Considered:** pass the stored configuration beside the working one through every command; compare every
  preimage with the same view of the stored configuration.
- **Chosen because:** the view is one function, and a client's file that changes between the read and the write
  then fails the same guard as a configuration that changed.
- **Applies to:** `store.Snapshot.MatchesConfiguration` and `Store.SaveVerifiedCheckpoint`, which take the view;
  `commit`, and `Dependencies.SaveVerifiedSnapshot` for the check. A preview compares what is stored with what it
  would save by the same function, so `aigw use` reports whether the configuration changes alike in its preview and
  its result.

### A choice is saved only in the transition that writes AIGW's record of the client's file

- **Rests on:** two reviews' runs. A model chosen in Codex, a saving command that writes no Codex file, the earlier
  model chosen again: the record had stayed behind, so status reported drift and `aigw sync` wrote the first choice
  back. And a model of a provider without a Token chosen in Hermes, a saving command, the working model chosen
  again because the first could not run, the Token stored: synchronization wrote the first choice over the last.
  Tests repeat both, for Codex and for Hermes, with a command that names nothing of the client, and the script does
  the first with the built program.
- **Considered:** rewrite the record when the choice is observed; write the record alone, in AIGW's file beside the
  client's; follow a choice only while the client's provider has a Token, which makes the working configuration
  change inside a command that stores that Token; refuse a command while a choice cannot be recorded; reconcile the
  followed client's files in the transaction, and save nothing of a choice whose files it does not write.
- **Chosen because:** the transaction already writes client files with their records and undoes them together, so
  the record needs no second writer, and observation stays read-only. A choice that is not saved loses nothing: it
  stays in the client's file, every command still works from it, and the next transaction that can plan the file
  records it, by writing the file or, while the provider of its selection has no Token, by the record alone (see "A
  choice is recorded with or without its Token, by a record-only write"). A refusal would stop a command about
  something else for a file that it does not name.
- **Applies to:** `recorded` in `internal/app/synchronization`, which `commit` and the previews share. A client
  whose choice the command does not touch joins the reconciled clients when its files can be planned; the result
  then names its files. Where one of its files cannot be planned, what is saved holds that client as stored.

### A choice is recorded with or without its Token, by a record-only write

- **Rests on:** the seventh review's eight runs (F1–F8), seven of them in the row "chosen" of the state table where the
  chosen model's provider has no Token that AIGW can use, and the measurements this redesign made: a Hermes whose
  choice's provider has no Token, a saving command: the stored selection and the record move to the choice and
  Hermes's file keeps its bytes, then `sync` after the Token writes it; Codex with two files of which one chose: the
  record of that file moves and its file keeps its identity, the other file's record waits for the projection that
  writes it; a failure after the record moves it back; a rollback to an unpinned selection that resolves to the
  chosen model records it; `aigw client edit hermes --target` of a client whose provider has no Token is refused.
- **Considered:** handle the waiting state on more paths, as versions four to seven did, with a reported
  configuration, a guard on the transition and a `shown` configuration per question; refuse to record a choice whose
  provider has no Token.
- **Chosen because:** rounds five, six and seven each handled the state on more code paths and each review found it
  on another, because the state exists for one reason: a choice was recorded only by writing the client's file, and
  a client's file is not written without its Token. Recording needs no client file — the file already shows the model,
  and only AIGW's record of it and the stored selection have to move — so the state is removed instead of handled: one
  configuration for every command, the working one, and a client without a Token is simply a client whose selection's
  provider has no Token.
- **Applies to:** `SelectionRecorder`, `PlanRecord` and `ApplyRecord` in `internal/adapter`, dispatched by
  `Registry.Plan` and `Registry.Apply` for the clients in `Dependencies.RecordOnly`; `hermes.PrepareRecord` and
  `codex.prepareCodexRecords`, which write the record alone, guarded by the snapshot that they read, with a receipt
  that restores it; `recordsAlone`, `projectionDependencies` and `SameSelection` in `internal/app/synchronization`,
  which send a client whose record is behind its file to the record-only write and compare selections as
  resolved runtimes, never by a binding's raw protocol; and `aigw config rollback`, whose `selections` names only a
  stored selection that it changes to another model than the file shows, and which records the choice where the
  restored configuration selects it.

### One sentence holds for every command, and one judge decides every client

- **Rests on:** the eighth and ninth reviews. The eighth found a record left behind a file that shows the stored
  selection; the ninth found a choice saved for a named client whose files could not be recorded (two Codex files,
  one missing, no Token, `aigw sync`). An exhaustive test written before the change failed in five cases on the
  code that the ninth review read, one of which no review had found (two files with different picks, no Token, an
  edit of the client's own setting). Nine reviews each sampled sequences; none enumerated them.
- **Considered:** refresh the record on one more path, as versions two to nine did; decide at the settlement which
  clients are saved by predicting what the plan will do, which is what failed; drop following without a Token.
- **Chosen because:** the sentence is small enough to state and to check for every case: after a committed
  command the record of each of a client's files names the model that the file selects, or that client's
  selection, files and records are exactly as before. It holds when one judge decides every client that a
  transition touches, with a Token or without: the ordinary projection's own admission. A client whose files are
  written is planned as before; a client that a command names and that waits for its Token is planned without
  being written; a client that no command names has its record brought up exactly when that plan is admitted; and a
  client whose plan is refused is left as it is stored, or refuses the command that names it. The Token then
  decides only whether the files are written, never what is saved or refused.
- **Applies to:** `recordsAlone` and `admitsWaiting` in `internal/app/synchronization` and `RecordPending` of the
  adapters, which ask the projection's `Plan`; `restoredClients`, `CommitRestore` and `PreviewRestore`, by which a
  rollback names every client whose stored binding it replaces; `FromStoredBinding`, from which an explicit
  selection takes its defaults and its report; and the three tests
  `TestEveryCommandLeavesEachRecordNamingWhatItsFileSelects…` (Codex on one file, Codex on two, Hermes) with their
  states and commands, which a new state or command joins before the code that handles it. `aigw sync` therefore refuses a client that waits for its Token
  when its files would be refused with the Token, where it passed over that client before. The plan, which for
  Codex starts the Codex program, runs only for a client whose record is behind its file and for a client that a
  reconciling command covers while it waits for its Token; every other command reads the file and its record once.

### A selection line has one reader, the file's own format

- **Rests on:** the tenth review. A model picked in Codex's file as a TOML literal string (`model = 'gpt-terra'`)
  was read as no choice: `readSelection` decoded the line's value as a JSON string, which a literal string is not,
  and took the empty result for "shows the recorded model", while the plan's admission read the same line as TOML.
  One fact had two parsers. The widened matrix failed in 26 runs on it, in the review's three sequences.
- **Considered:** teach the second reader the literal string; normalise the line before reading it.
- **Chosen because:** Codex's file is TOML, so TOML decides what a line assigns. `selectionValue` reads a root
  selection line as TOML and is the only reader; `selectionHasValue` rests on it, and the second reader is deleted.
  Every spelling that TOML gives a string names the same model, the ones that nobody listed included.
- **Applies to:** `selectionValue` and `selectionHasValue` in `internal/adapter/codex/config.go`, `readSelection` in
  `adoption.go`, the table test `TestASelectionLineIsReadAsTOML`, and the matrix's dimension of spellings: Codex's
  basic string, literal string, no spaces and trailing comment, and for Hermes, whose file is read by its YAML
  parser already, the model or the provider as a plain, single-quoted or double-quoted scalar.

### An edit of a client's own settings is judged against what is stored

- **Rests on:** the tenth review. With a stored forwarding, a choice of a model of another upstream ends the
  forwarding in the configuration that every command works from. `aigw client edit hermes --direct` compared its
  edit with that configuration, found nothing to change and saved nothing; the stored selection kept the
  forwarding, which came back when the operator picked the stored model again. The widened matrix also asks what
  a command does that succeeds and changes nothing, which the checker had accepted from any command.
- **Considered:** commit every edit, changed or not, so that every successful command records a pending choice;
  compare field by field what the edit names with the stored binding.
- **Chosen because:** the edit is about the client as it now stands, and that differs from what is stored exactly
  when the client's file chose a model or the edit changes a setting. So the edit is compared with the stored
  binding of the client (`FromStoredBinding`): after a choice it always has something to save, the choice, and
  saves it with what the edit asks for. An edit that asks for what is stored already, while no file of the client
  chose a model, has nothing to save: it commits nothing and changes nothing, as a preview does. The specification
  already said that a command records a choice when it saves a change; a command that saves nothing leaves the
  state as it found it, which every report still describes.
- **Applies to:** `runEdit` in `internal/cli/client/edit.go`; the matrix's command `end forwarding` with its
  declaration `idle`, under which a run must leave the configuration, the files and the records untouched, and its
  check `leaves`, which reads the stored binding; `TestAForwardingEditOfAClientThatWaitsForATokenActsOnTheChosenModel`,
  rewritten from the old outcome to this one.

### A removal beside a choice that cannot be recorded is refused in the file's words

- **Rests on:** the tenth review's finding 4. While a file of a client is refused, the client's choice stays in
  its file and the client is saved as stored. Removing the model that the stored selection names then passed the
  removal guard, which reads the working configuration, and failed the validation of what would be saved with
  "names the unknown model" and `aigw check` as its step.
- **Considered:** make the guard read the stored configuration too, which would also refuse the removal that a
  recordable choice allows.
- **Chosen because:** the transaction knows why the client is left: the plan of its files was refused. It returns
  that refusal, which names the file and its step, wrapped in a sentence that says which selection is kept.
- **Applies to:** `recorded` and `planRefusal` in `internal/app/synchronization/follow.go`, and the regression test
  of the tenth review's removal probe.

### A Token that cannot be shown leaves its client waiting, and the choice is still recorded

- **Rests on:** the eleventh review's finding 4. `credentialReadyClients` returned the error of a credential store
  that could not say whether a slot was there, as a locked keychain cannot; the plan then failed, the transaction
  read that failure as a refused file, and a command about something else succeeded with AIGW's record left behind
  the choice. The Token decided the outcome, which is the third defect family.
- **Considered:** refuse such a command, so that the store's failure reaches the operator; treat the client as
  written, which would write a file naming a reader that cannot return a Token.
- **Chosen because:** the three ways in which a Token is unusable — absent, empty, unobservable — already have one
  answer in `tokenUsable`: the client waits for its Token. An unobservable slot is a fourth reading of the same
  condition, not a different failure of the command. A waiting client is planned with the dependencies of a
  record-only write, which reads no Token, so the choice is recorded and the file stays as it is. This also
  settles `CredentialEntrypointPlan`, which now answers Unchanged instead of the store's error.
- **Applies to:** `tokenUsable` and `credentialReadyClients` in `internal/app/synchronization/synchronizer.go`;
  `TestZZ11AChoiceWhileTheTokenCannotBeRead` (both ways of refusing the presence).

### The files a client drops are restored without a Token, and the files it keeps wait for one

- **Rests on:** the eleventh review's findings 2 and 3. A rollback that restored another file of Codex dropped the
  current file; with no Token the client was not planned, so the dropped file kept AIGW's settings and its record,
  which no later command visits. And `aigw client edit --target` refused the files that the client already had
  whenever no Token was usable, although that edit drops nothing and its record needs no Token.
- **Considered:** refuse a rollback that drops a file while the client has no Token (the Token would then decide
  the refusal); plan the whole client as if it had a Token (it would write a file naming a reader that returns
  none).
- **Chosen because:** a file that a client no longer has cannot keep AIGW's settings, and restoring it reads the
  file and its record and never a Token — the same reason a withdrawal needs none. So a client that is not written
  but loses a file joins the transition as a third kind, `Withdrawn`: its dropped targets alone are planned, through
  the restore transition the adapters already have, and its kept targets wait for its Token as before. For the
  edit, the refusal now depends on what the edit does and not on which flag it used: only an edit that drops a file
  loses one that could keep AIGW's settings, so only that edit waits for the Token.
- **Applies to:** `adapter.Dependencies.WithdrawOnly`, `planClient` and `applyClient` in `internal/adapter/record.go`,
  `reconciliationInputs` in `internal/adapter/codex/adapter.go`, `hermesPlans` in `internal/adapter/hermes/adapter.go`,
  `withdrawnClients`, `reconciled` and `projectionDependencies` in `internal/app/synchronization/record.go`, and
  `admitTargets` in `internal/cli/client/edit.go`; `TestZZ11ARollbackToOtherFilesOverAChoice` and
  `TestZZ11TheSameFilesAreRefusedWithoutTheToken`.

### The matrix's checker is its own subject of test

- **Rests on:** the eleventh review's seven holes. Each was a path on which a product with that fault would have
  passed the matrix: a command's own setting that was never stored, a binding that differed between twins in
  everything but the model, a preview that wrote a backup beside the configuration, a stored selection of another
  provider read by model ID alone, a file that vanished under the command, a rollback checked by the model alone,
  and a withdrawal that left the client enabled.
- **Considered:** read the checker and reason about the seven, which is what the tenth review did for its own
  gaps; add only the cases that the product could reach today.
- **Chosen because:** a checker's defects are invisible in its passing runs, so each needs an outcome no command
  produces, built by hand and given to the checker. `faults` returns what the checker finds instead of reporting
  it, so one probe can feed it a wrong outcome and require a fault; `holds` and `twinned` report those faults for
  the matrix. The probes stay as regression tests of the checker itself.
- **Applies to:** `faults`, `committedFaults`, `withdrewFaults` and `twinnedFaults` in
  `internal/cli/acceptance/selection_invariant_checks_test.go`; the seven inverted probes in
  `zz11_review_checker_test.go`.

### An adapter admits a file that shows the selection the transition starts from

- **Rests on:** a transition now starts from a configuration that names the client's choice while AIGW's record of
  the file still names the earlier model. A table test runs every state of Hermes's model block (a listed model, an
  unlisted one, another entry's model with the earlier address, an edited address, a removed mode or model, a
  provider that AIGW does not own, no block, a block that is one word, a listed model beside an edited entry)
  against following, an explicit selection, a transition and a withdrawal; Codex's ownership tests run the root
  model against the recorded, the new and the starting model.
- **Considered:** rewrite AIGW's record of the file when the choice is observed; admit the starting selection in the
  ownership guard.
- **Chosen because:** the guard already admits the model about to be written, and the starting model is the same
  kind of proof. Hermes's block is classified once: a listed model of an owned entry is a choice; an unlisted model
  or a provider that AIGW does not own is a selection that AIGW does not follow; an address or mode that differs
  from the entry is an edit of an endpoint and stays an outside edit, for an explicit selection too.
- **Applies to:** `removeCodexProjectionTransition`; `classifySelection`, `ObserveSelection` and `PrepareTransition`
  in `internal/adapter/hermes`, which replace the adopted and authorized variants.

### One owner says how Codex's files stand against the selection

- **Rests on:** three reviews' runs. With one file naming a model that was not added and another holding an edited
  provider table, every surface named the restoring command, which the edit then refused, and in the other order
  of the files status named `aigw sync`, which was refused too. A file that showed the followed model passed
  inspection although another managed value had drifted. And `aigw check codex --depth client` failed after a
  choice, because it compared the file with the strict projection where the inspection no longer did. Tests cover
  the two files in both orders on every surface, files that agree, files that disagree, an inspection of a followed
  choice beside a drifted retry budget, and the client-depth check after a choice.
- **Considered:** let the registry replace any conflict by the selection's refusal when a model differs; adapt each
  place that compares a file with the selection; one observation of all of Codex's files, read by following, by the
  inspection and by the check that runs the client.
- **Chosen because:** a recovery must run on the state that it is given for, and three places that each decide what
  a choice is disagree sooner or later, as they did twice. An outside edit in any file leaves the plan's own
  refusal standing and decides the client's next step, whichever file comes last.
- **Applies to:** `observeChoice`, `followedChoice`, `showsSelection` and `awaitsRecord` in
  `internal/adapter/codex`. A file shows the selection when it is AIGW's projection of it, or, after Codex's own
  choice, what AIGW last wrote with that one value exchanged. A file that still holds what AIGW last wrote, while
  another file chose, awaits the record: the inspection accepts it, and the client-depth check runs Codex on the
  file that shows the choice.

### A selection that AIGW does not follow is a typed refusal with a step that works

- **Rests on:** the run of 2026-11-20: for Codex the Fix, parsed from the error's text, was
  `aigw use codex team/<the unknown ID>`, which fails when the provider does not serve it and which followed the
  foreign value on 2026-11-19; for Hermes the Fix was "Undo the outside edit", and `aigw use hermes` refused too. A
  script runs the Fix as printed and then `aigw sync`.
- **Considered:** keep the Fix that adds the named model; overwrite the value with the recorded model; refuse with
  both commands, the restoring one as the Fix.
- **Chosen because:** the restoring command runs whenever the selection is the file's only difference, and a Fix
  must run; overwriting would discard a model that the person typed on purpose. A model ID that a shell would not
  pass through unchanged gives the step in prose, by the rule that every other command line follows.
- **Applies to:** `SelectionError`, `RecordedSelection` and `SelectCommand` in `internal/adapter`; the registry
  replaces a plan's conflict, and an inspection's issue, by that refusal when the file's only difference is such a
  selection. The shell-word rule moves from the command layer to `internal/platform`, where the adapters can reach
  it.

### A refusal before anything was written says so, from a command and from its preview

- **Rests on:** `aigw config rollback` wrapped every failure of its transaction as "did not complete; the
  configuration or client files may have changed", with `aigw check --depth local` as the Fix, which hid the
  restoring step and was untrue of a refusal that wrote nothing; and a command's refusal read "sync synchronization
  preflight failed; configuration and client files were unchanged: …" where its preview gave the refusal alone.
  Tests read problem, impact and Fix from `sync`, its preview, another command and a rollback.
- **Considered:** let the rollback pass a selection's refusal alone; one type for a transition that was refused
  before any write, returned by the one planning step that a command and its preview share.
- **Chosen because:** that nothing changed is a fact of where the refusal arose, not of its cause, so one type
  carries it for every cause, and the presentation states it once. A fix that an outer error states still wins, as
  for a setup before a configuration exists.
- **Applies to:** `PreflightError` and `planned` in `internal/app/synchronization`, which `commit` and the previews
  now share; `terminal.UnchangedImpact` and the problem that `internal/terminal` presents; `aigw config rollback`,
  which passes such a refusal on. The refusal's text no longer carries the words "synchronization preflight failed".

### A rollback restores the selections of the restored configuration, and names them

- **Rests on:** the second review's run: after a model chosen in Codex, `aigw config rollback` wrote the restored
  configuration's model over it and reported nothing of it. Tests read the preview, the result and Codex's file.
- **Considered:** keep the client's choice across a rollback, which needs to know which of the restored selections
  the person meant; refuse a rollback while a choice is unrecorded; restore every selection and name each one that
  changes with the model that it replaces.
- **Chosen because:** a rollback asks for the earlier configuration, its selections included, as `aigw use` asks
  for one model; what would be wrong is doing it silently. The owner's rule of 2026-11-20 names `aigw use` alone as
  the command that changes a client's model, so the reading was put to the owner, who confirmed it on 2026-11-20.
- **Applies to:** `selections` in the rollback's document and the section "Selections" of its human output, in the
  preview and in the result: each stored selection that the rollback changes to another model than the client's file
  shows, by resolved runtime, so that a restored selection without a protocol that resolves to the file's model is
  the choice and is recorded and named as followed, not restored. A rollback of a client whose selection's provider
  has no Token saves the restored selection as pending and brings the record to what the file shows, so that a later
  pick, the model that the record named before included, is a choice.

### One record of what an invocation followed, reported ahead of the next step

- **Rests on:** the first version put the report into the client-file rows, and its review listed the commands
  whose results never pass through those rows; the second kept a list of saving commands beside the transaction and
  printed after the command's own output; the third printed the section after a command had failed, when nothing
  was saved, because the command's own preview had settled the record. Tests read the report of a saving command, a
  preview, an explicit selection, `status`, a passing and a failing `check`, bare `aigw`, a command that reads
  without reporting selections, and a saving command that fails.
- **Considered:** a field and a line in each command's own result; a list of saving or reporting commands at the
  root; one record that the load fills and that the transaction, or its preview, settles, with the command layer
  deciding what an invocation names.
- **Chosen because:** what was followed is a fact of the invocation, and only the transaction knows what it
  recorded: not the client that an explicit selection names, not a client whose selection the transition replaces,
  not a choice that it left in the file, and no returning command for a model that the transition removed.
- **Applies to:** `synchronization.Follows` holds what the load observed and what the last transition, or its
  preview, recorded. `invocation.FollowedReport` decides what the invocation names: a command that reports what it
  reads (`ReportedConfiguration`: status, check, bare `aigw`) names every observed choice,
  also when it fails; any other command names what its transition recorded, and nothing when it failed. The
  renderer writes the section ahead of "Next", as a sentence and the returning command on a line of its own;
  `terminal.WriteDocument` adds `followed_selections`, whose entries carry `client`, `model`, `earlier`, the two
  protocols for a client that speaks several, `returns_with` while the configuration holds the earlier model and a
  shell passes its ID unquoted, and `forwarding_cleared` or `retries_cleared` when such a setting ended with the
  earlier upstream endpoint or provider. `aigw sync` loses `adopted_selections`.

### Claude Code's own model is shown, not recorded

- **Rests on:** `InspectSettings` already recognises a settings file whose only difference is its `model`, and
  returned a boolean; the value is in scope there. Tests read the status row and both documents.
- **Considered:** record the value as the selection; show it beside AIGW's selection.
- **Chosen because:** the value may be an alias that names no model of a provider, so it cannot be a selection;
  showing it tells the person which model Claude Code sends.
- **Applies to:** `internal/adapter/claude`, `adapter.Status` with the one wording of the note, the status row and
  the check's detail, and `client_model` in both documents.

## Risks / Trade-offs

- A stale write of a running client is followed as a choice. The followed model, the earlier one and the returning
  command are printed, and the next Change words what a running client needs.
- A command that changes the settings of a client whose selection's provider has no Token saves them on that
  selection, and the client's file waits for the Token: a forwarding set then reaches the chosen model's upstream,
  which the next synchronization writes.
- A command about something else now writes the file of a client whose choice it records: for Codex it puts AIGW's
  comment back on the model line and the chosen model's label into the provider's name. The result names the
  client's files.
- A file of the followed client that cannot be planned keeps a command about something else from recording the
  choice, not from succeeding. A file that is planned and then cannot be written, as in a folder without write
  permission, fails the command, as any write does.
- The command that returns to the earlier model is printed while the configuration holds that model. When that
  model's provider has no Token, `aigw use` asks for one in a terminal and refuses elsewhere, naming the Token.
- Beside an outside edit, a model that was not added costs two recoveries in turn: the edit first, then the
  restoring step.
- A model of a provider whose Token is gone, chosen in the client, is `aigw status`'s and every saving command's
  selection, waiting for the Token; the choice is recorded with the record of the file, and the file is written once
  the Token is stored.
- Every command now reads each enabled client's files and AIGW's records of them before it runs. A file that cannot
  be read leaves its client as stored.
- The clients' writes are imitated in every test of this Change; the next Change's measurement with the real
  clients may change the conditions of following.

## Migration Plan

No data changes. A script reads `followed_selections` where it read `adopted_selections`, receives the restoring
command as the Fix of a selection that AIGW does not follow, finds `selections` in the rollback's document, and no
longer finds the words "synchronization preflight failed" in a refusal's text, whose `impact` says that nothing
changed.
