## Context

`aigw setup --from` merges the team file, reads Tokens, validates each, selects models for clients without a
selection and writes everything in one transaction (`internal/cli/setup/team.go`). In a terminal without
`--provider` it prompts for the missing Token of each provider that a client's first usable recommendation would
select, which in the shipped file is DMXAPI alone. The secret prompt refuses an empty answer, and any failed
validation returns before the write. Reproduced on 2026-11-19 with the installed 0.8.2 in an isolated home with the
file backend: Enter alone ended setup with "this value cannot be empty"; a Token that DMXAPI does not accept ended it
with "Token validation failed: Codex authentication was rejected (HTTP 401)" and "The team file was not applied".

## Goals / Non-Goals

**Goals:**

- Any subset of Tokens, including none, a wrong one or a skipped one, still applies the team file and configures every
  client that the accepted Tokens can serve.
- A person is asked only for Tokens they say they have, and a rejection says which provider rejected what.

**Non-Goals:**

- Guided setup without `--from`, `aigw provider add` and `aigw provider rotate`: each asks for one provider's Token,
  which the person named.
- Choosing a model that no recommendation names: the team file stays the one source of recommended selections.

## Decisions

### The team file is applied whatever becomes of each Token

- **Rests on:** the owner's rule of 2026-11-19; the reproduction above; the transaction already takes a Token map, so
  the write needs no Token that was not accepted.
- **Considered:** keep refusing on any failed Token; apply the file and skip the failed provider; apply the file only
  when at least one Token is accepted.
- **Chosen because:** the team file's providers, models and recommendations are useful without any Token, which the
  existing "No Token or client is present" scenario already requires; a failed Token is a fact about one provider.
  The one write keeps its atomicity: the configuration, the accepted Tokens and the client files still commit
  together or not at all.
- **Applies to:** `teamSetup.apply`; the report gains the Token state `rejected` with the reason and the provider's
  `aigw provider rotate` step.

### A terminal asks which providers' Tokens the person has

- **Rests on:** the user's report: asked only for DMXAPI, they typed the AIHubMix Token they held. A Token belongs to
  exactly one provider and is validated only against that provider. Every prompt of `internal/terminal` reads the
  same in accessible mode (numbered answers on plain lines, as `Select` does).
- **Considered:** prompt for each missing Token in turn with Enter to skip; ask which providers first, then prompt
  for those; prompt only for the recommended provider, as now.
- **Chosen because:** one multiple-choice question names every provider at once, so the person matches their Token
  to its provider before typing it, and choosing none is an explicit answer rather than an empty secret. The list
  holds the team file's providers whose Token is missing, in the file's order; a provider whose store cannot tell, or
  whose Keychain item macOS refuses, keeps its own fix and is not listed.
- **Applies to:** `invocation.Prompter` gains `MultiSelect`; `terminal.Prompt` implements it with huh's multi-select
  and, in accessible mode, a numbered list read as space- or comma-separated numbers, Enter alone choosing none.

### Enter alone skips a provider, and a held Token survives a failed one

- **Rests on:** the first review of this Change found that a person who ticked a provider by mistake could leave its
  prompt only with a wrong Token or by cancelling, which discarded the file; that malformed text and an accessible
  question's three unmatched answers ended setup; and that a refused piped Token reported a provider whose stored Token
  still worked as having none.
- **Considered:** route cancellation to a skip; a skip choice on every prompt; a prompt that accepts an empty answer.
- **Chosen because:** Enter alone is the one answer a person gives without thinking, and the prompt says it skips;
  Ctrl-C keeps its meaning of stopping the command. Malformed text gets the same enter-again-or-skip choice as a
  refused Token, and an accessible question that names nothing chooses none. A refused piped Token stores nothing, so
  a Token that the store held stays connected and its row says that the new one was not stored. A validation without
  a verdict (no answer, 408, 429 or 5xx) is reported as not verified rather than not accepted, and a terminal offers to
  try the same Token again. Piped input that is not one Token is the script's input error and stays refused before any
  write.
- **Applies to:** `invocation.Prompter.OptionalSecret`, `teamSetup.readToken` and `apply`; the JSON document goes to
  schema version 4 for its new state, reason, `error` and `impact`.

### A Token that is not accepted is entered again, skipped, or reported

- **Rests on:** a person at a terminal can correct a paste; a script cannot answer a question; `command-line` defines
  exit 1 for a command that fails its request and `terminal.Presented` ends a command whose document is written.
- **Considered:** retry until accepted; skip silently; ask again or skip; for a piped Token, refuse before the write as
  now, or write and exit 1.
- **Chosen because:** in a terminal, the choice between entering the Token again and skipping the provider keeps the
  person in control and never loops without their answer. A piped Token is the script's explicit request: the write
  goes ahead for everything else, and exit 1 with the provider reported as `rejected` tells the script that its
  request failed. Every validation failure, a 401 or 403, another status or no response, takes the same path; its
  reason says which.
- **Applies to:** `teamSetup.acquire`, which now validates as it reads.

### A validation failure names the provider

- **Rests on:** the probe validates a provider's Token through the protocol of a client that speaks it, and its errors
  named that client ("Codex authentication was rejected", "Codex endpoint returned HTTP 404"), which the user read as
  AIGW's own misjudgement.
- **Considered:** keep the client's name; name the provider; name both.
- **Chosen because:** the provider is the party that answered. A refusal names the provider by its label and keeps the
  status (`DMXAPI did not accept the Token (HTTP 401)`); another status or no answer names the endpoint by its
  protocol and provider (`the OpenAI Responses endpoint of DMXAPI returned HTTP 404`). The client that carried the
  request still decides the protocol.
- **Applies to:** `probe.TokenRejectedError`, `EndpointStatusError` and `UnreachableError`, so every command that
  validates a Token says the same.

### The shipped file recommends every provider that serves a recommended family

- **Rests on:** `SelectModelsForConnectedProviders` fills a client from its recommendations alone; the shipped file
  recommended DMXAPI first and UCloud second, so an AIHubMix Token configured no client although AIHubMix serves the
  same families (Claude Opus 5.5 on Anthropic Messages, GPT-6.2 Sol on Responses).
- **Considered:** fall back in code to any model of the recommended family on a connected provider; list every
  serving provider in the file's recommendations.
- **Chosen because:** a recommendation also carries the protocol and the client's options, which a code fallback
  would have to guess; the file stays the one place that decides. A test reads the file's structure, not its models:
  for each client, every provider that has a usable model of the first recommendation's family appears among the
  client's recommendations.
- **Applies to:** `manifests/team.toml` and a test in `internal/store`. The label example of
  `progressive-team-onboarding` that claimed `claude-code-opus-5-5` derives `· claude-code` from the family
  `claude-opus-5-5` is corrected: that ID neither begins nor ends with that family's ID.

### An exported team file keeps Taplo's layout

- **Rests on:** `TestTeamManifestUsesNativeExportLayout` requires the shipped file to equal `aigw config export`'s
  bytes, and the repository's Taplo check (`column_width = 100`) expands an array that passes 100 columns. A third
  Hermes recommendation made one line 138 columns wide, which no file could satisfy for both.
- **Considered:** shorten the line by dropping the recommendations' protocol; expand wide arrays in every encoded file;
  expand them in a team file export only.
- **Chosen because:** the protocol carries meaning, and every further provider widens the line again. A team file is
  the one encoded file that a repository formats; `aigw.toml` keeps one line per value, so no user's configuration is
  rewritten. The export writes each element of a wider array on its own line, indented two spaces with a trailing
  comma, as Taplo does.
- **Applies to:** `store.ExportTeamFile`. A test of the shipped recommendations that listed their models is replaced by
  one that checks every choice resolves for its client and never replaces an explicit selection.

## Risks / Trade-offs

- A provider whose model catalog answers without a Token cannot prove a Token: on 2026-11-19 AIHubMix's
  `/v1/models` answered 200 to a request without a Token and to a request with an invalid one, so its validation
  accepts any Token, as it did before this Change: a run of this lane's build stored an invalid AIHubMix Token as
  accepted. `aigw check` sends an inference request and catches such a Token. The review of this Change noted that
  recommending AIHubMix for every client gives that weak validation more weight; a validation that proves a Token on
  such a provider is a separate decision, raised with the owner after this release.
- A script that relied on a rejected piped Token leaving the configuration untouched now finds the team file applied;
  its exit status is unchanged.
