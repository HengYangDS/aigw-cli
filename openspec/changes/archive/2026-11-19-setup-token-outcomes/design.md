## Context

`setup-partial-tokens` made `aigw setup --from` apply a team file whatever becomes of each Token, and `team-suheai`
added a fourth provider; both are archived in this lane and wait for the 0.8.3 tag. A fresh review of the lane's
range read the Token flow (`internal/cli/setup/team_tokens.go`), its report, the prompts and the records, and found
the gaps that the proposal lists. Nothing here changes a JSON field, a state or the transaction.

Two statements of the archived Changes are corrected here and not in the archive: `team-suheai`'s proposal gives the
prefixed models' labels as `· Claude Code`, where the file and the specification carry `· CC` and `· CDX`; and its
design claims a real-client check of each client on its Suhe.AI recommendation, where Claude Desktop was checked at the
`model` depth.

Measured on 2026-11-19 at 19:56 CST with one made-up Token, one request for each provider's model catalog, which is the
request that validates a Token:

| Provider | Made-up Token as Bearer | Made-up Token as `X-Api-Key` | No credential |
| -------- | ----------------------- | ---------------------------- | ------------- |
| DMXAPI   | 401                     | 401                          | 401           |
| UCloud   | 401                     | 401                          | 200           |
| AIHubMix | 200                     | 200                          | 200           |
| Suhe.AI  | 401                     | 401                          | 401           |

At 20:50 the same made-up Token got 401 from all four, AIHubMix included, on `/v1/responses`, `/v1/chat/completions`
and `/v1/messages` with an empty JSON body; what a valid Token gets there was not measured. The readings are in the
canonical checkout's `build/evidence/token-proof-20261119/`.

## Goals / Non-Goals

**Goals:**

- No answer at a Token prompt discards the team file or forgets what the provider said.
- The report of a run matches what the person did, and the records match what the code does.

**Non-Goals:**

- Proving a Token on a provider whose catalog answers any Token; the next Change does that.
- Handling an interrupt signal during a validation: no command wires one to its context, and nothing is written
  before the transaction.
- A preview without a terminal that names a provider without a Token: it plans as if the Token were accepted and
  names the run without `--token-stdin`, which that run refuses. This predates the lane and changes a JSON preview's
  `next_action`, so the next Change takes it.

## Decisions

### A provider keeps its last verdict however its prompt is left

- **Rests on:** the scenario "A Token typed in a terminal is not accepted" says that a provider skipped after a
  refusal is reported as not accepted. `readToken` forgot the refusal when the person chose to enter the Token again
  and then pressed Enter alone, and returned the `Select` error of answers that named no choice, which ended setup
  before the write.
- **Considered:** ask the question again until an answer names a choice; treat no choice as a skip.
- **Chosen because:** the accessible selection already bounds its unmatched answers, and the provider question takes
  the same ending as no choice; a skip loses nothing, since the fix of a provider without a Token is printed. Text
  that is not one Token says nothing about the provider, so it leaves the verdict that an earlier Token got, or none.
  Setup says what it made of such answers, at this question and at the provider question, because the next prompt
  would otherwise follow three refusals of an answer without a word.
- **Applies to:** `readToken` in `internal/cli/setup/team_tokens.go`.

### A cancelled prompt says that no accepted Token was stored

- **Rests on:** Tokens are written by the one transaction after every prompt, so a cancel at a later prompt drops the
  Tokens that this run accepted; the cancelled prompt's problem said only "Nothing changed".
- **Considered:** write what was accepted so far; keep the cancel and say what it dropped.
- **Chosen because:** a person who cancels expects no write, and setup has one commit boundary. The sentence is added
  by the owner of the cancelled-prompt problem, so that the title, the fix and the exit status stay its own.
- **Applies to:** `invocation.Prompted`'s error, `invocation.CancelledPrompt`, and `acquire`.

### A named provider is asked once and gets its next step when skipped

- **Rests on:** the requirement defers a provider and names its fix; the report left a named provider's row without
  one, because in a preview that Token is the command's own input. A provider named twice was read twice, and a second
  answer could mark a stored Token as rejected.
- **Considered:** refuse a repeated name; read each name once.
- **Chosen because:** a repeated name asks for nothing new, and a refusal would be the only one that a harmless
  repetition causes. A run that has asked and got no Token names the step that stores one, as for every other provider;
  a preview keeps the row without a step.
- **Applies to:** the setup command's request and `document` in `internal/cli/setup/team_report.go`.

### A preview in a terminal says that the run asks

- **Rests on:** a preview reads no Token and asks nothing, so it plans with the Tokens that are present and takes a
  named provider's as accepted; the run in a terminal asks which providers' Tokens the person holds.
- **Considered:** ask in the preview as well; say what the run does.
- **Chosen because:** `--dry-run` promises no input and no write. One line of the human preview says that the run
  asks; the JSON preview, which a script reads and which never prompts, is unchanged.
- **Applies to:** `preview` and `write` in `internal/cli/setup`.

### The recommendation order is checked from the file's structure

- **Rests on:** the rollout guide says that each client recommends one model on every provider that serves its
  family, in the file's provider order; the tests checked the coverage and not the order, and counted a model as
  unusable on any resolution error, which a model with two usable protocols also gives.
- **Considered:** list the expected recommendations in the test; read the order from the file.
- **Chosen because:** the owner's rule of 2026-11-18 is that a test checks structure and never repeats a configured
  value. For each client, the providers of its choices are a strictly increasing subsequence of the file's
  providers, and a model serves a client when the client resolves it under the default or any one of its protocols.
- **Applies to:** `internal/store/team_recommendations_test.go`.

### The records state the validation's limit; they do not hide it

- **Rests on:** the table above. AIHubMix's catalog answers 200 to any Token, so its validation accepts a wrong one,
  as it has since AIHubMix was admitted. From this release the shipped file recommends AIHubMix for every client,
  which the file had kept for explicit selection since the correction of 2026-10-02; that correction's reason is in
  no record that this lane could find.
- **Considered:** take AIHubMix out of the recommendations again; add a control request to the validation in this
  release; state the limit and build the proof as the next Change.
- **Chosen because:** without the recommendation a person who holds only an AIHubMix Token gets no client
  configured, which is the report this lane answers. A validation that tells a proved Token from an unproved one
  changes five commands and `secret-storage`, and deserves its own design and review, not the tail of a release. A
  wrong AIHubMix Token fails at the first request. `aigw check` shows it once a client selects an AIHubMix model,
  since it sends an inference request for each client's selected model alone; for any other case the records name
  `aigw check <client> --model <provider/model>`.
- **Applies to:** `CHANGELOG.md`, the rollout guide and the route evidence, which also records that the 0.8.3 file
  supersedes the order of 2026-10-02.

## Risks / Trade-offs

- A person who pastes another provider's Token at the AIHubMix prompt is told that it was accepted, and gets every
  client without a selection configured on AIHubMix; the first request fails. Until the next Change, the guide and the
  changelog name the limit and the check that shows it.
- Claude Desktop on AIHubMix and on Suhe.AI was checked at the `model` depth, the deepest that `aigw check` runs for
  it, because its `client` depth needs acceptance inside the app; Claude Code, Codex and Hermes at the `client` depth.
  The records now say so.
- Whether Suhe.AI's undated `deepseek-v4.1-flash` serves the build that other providers list as `-1017` cannot be
  read from outside; Suhe.AI lists no dated ID, so the per-provider rule keeps the model that it serves.
