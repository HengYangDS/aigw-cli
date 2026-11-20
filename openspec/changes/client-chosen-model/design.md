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
earlier model again lost that choice once the Token was stored. The decisions below are the fourth version.

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
  stays in the client's file, every command still works from it, and the next transaction that can write the file
  records it. A refusal would stop a command about something else for a file that it does not name.
- **Applies to:** `recorded` in `internal/app/synchronization`, which `commit` and the previews share. A client
  whose choice the command does not touch joins the reconciled clients when its files can be planned; the result
  then names its files. Where one of its files is refused, what is saved holds that client as stored.

### A choice that waits for its Token is reported and not worked from

- **Rests on:** the fourth and fifth reviews' runs. While Hermes's chosen model waited for its Token, the command
  `aigw client edit hermes --via <address>` saved the choice without its record; the fix that put the stored
  selection back after the command then dropped `--direct`, which the follow had already cleared, and saved a
  forwarding to an upstream that the client no longer used. A test edits a waiting Hermes with `--direct` and
  `--via`, then stores the Token and synchronizes.
- **Considered:** repair what the command saved, field by field, after it ran; refuse an edit of a waiting client;
  work from the stored selection for a client whose chosen model waits for its Token.
- **Chosen because:** a command's intent is whole only in the configuration that it starts from, so the repair
  after it cannot tell an explicit `--direct` from a forwarding that the choice ended. Working from the stored
  selection gives every command the configuration that it saves, and the choice loses nothing: it stays in the
  client's file, and the first command after the Token is stored records it. A refusal would break the owner's rule
  that no command fails because of a model chosen in a client.
- **Applies to:** `Synchronizer.Working`, which leaves such a choice out and marks it as waiting, and
  `Synchronizer.Reported`, which `status` and `check` read, so that they name the model that the client uses. The
  preimage keeps the waiting state that the command read, so a command that stores the Token itself, such as
  `aigw use --token-stdin`, compares like with like. Once the Token is stored, the recorded choice ends a forwarding
  that reaches another upstream, as any choice does, and the result says so.

### A waiting client has one answer per question, read from one place

- **Rests on:** the sixth review's four runs against a Hermes whose chosen model waited for its Token:
  `aigw client list` named the stored selection while `aigw status` named the file's model; `aigw model remove`
  removed the model that the file selects, and the next synchronization failed on the file; `aigw client disable`
  left AIGW's entries and record in the file; a rollback saved the chosen model without its record and named it
  followed. Each is a cell of one table, the lane's state table, and five tests now hold its row "chosen, waiting".
- **Considered:** repair each of the four commands where the review found it, with a guard in `model remove` and
  another in `provider remove`; make the working configuration follow a waiting choice after all; answer each
  question once, where every command already passes.
- **Chosen because:** six versions each repaired the path that a review named, and the next review found the same
  state on another path. The state has three questions, and each now has one owner. Which model does the client
  use: the reported configuration, read by every command that saves nothing. What may not be taken away: the
  transaction refuses a transition that leaves a waiting choice without its model, protocol or endpoint, so a
  removal, an edit of the model's protocols, a pruning team file and a rollback meet one guard, where a guard per
  command would have left the next command out. What do the client's files show: the transition's `shown`
  configuration, the working one with each waiting choice followed, from which the adapters plan and write.
  Following a waiting choice in the working configuration would bring back the fifth review's finding.
- **Applies to:** `Dependencies.LoadReportedConfiguration` in `client list`, `model list`, `model show`,
  `provider list` and `provider show`, which name no followed selection; `servesChoice`, `shownBy` and `recorded`
  in the synchronizer, where a withdrawal names a waiting client for its adapter and any other transition keeps it
  out and puts its stored selection back when the transition would save the choice; `keeps`, which reports no
  waiting choice as recorded.

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
  preview and in the result; no followed selection is named for a client whose selection the rollback replaces. A
  client whose file the rollback does not write, because it waits for its Token, keeps a model chosen in it; for
  that client the rollback names only a change of the stored selection.

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
  preview, recorded. `invocation.FollowedReport` decides what the invocation names: a command that reads the
  configuration to report it (`ReportedConfiguration`: status, check, bare `aigw`) names every observed choice,
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
- While a client's chosen model waits for its Token, a command that changes the client's settings changes them on
  the stored selection, so a forwarding set then reaches the stored model's upstream, and the choice, recorded once
  the Token is stored, ends it if its upstream differs. The edit's result names the stored model, and the recording
  names the forwarding that ended.
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
- A model of a provider whose Token is gone, chosen in the client, is shown by `aigw status` as that client's
  selection, waiting for the Token, and is not saved; a saving command leaves the stored selection as it is.
- Every command now reads each enabled client's files and AIGW's records of them before it runs. A file that cannot
  be read leaves its client as stored.
- The clients' writes are imitated in every test of this Change; the next Change's measurement with the real
  clients may change the conditions of following.

## Migration Plan

No data changes. A script reads `followed_selections` where it read `adopted_selections`, receives the restoring
command as the Fix of a selection that AIGW does not follow, finds `selections` in the rollback's document, and no
longer finds the words "synchronization preflight failed" in a refusal's text, whose `impact` says that nothing
changed.
