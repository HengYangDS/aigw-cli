## Context

See `proposal.md` for the motivation and the measured inconsistencies. This
design depends on these facts about the source at the head of
`semantic-architecture` stage 5:

- Each top-level command has one package under `internal/cli/<command>`.
  Behavior that several commands share lives in `internal/cli/invocation`, and
  use cases that several commands share live in `internal/app`.
- Each command package defines its JSON document as a Go struct and writes it
  with `terminal.WriteJSON`. The generic failure is `{ok: false, Problem}` in
  `internal/terminal/errors.go`, and `Renderer.Next` renders one command.
- `cmd/aigw/testdata/script/*.txtar` runs the real program for every command,
  with per-platform expectations. `cmd/aigw/commands_test.go` requires each
  command's script to keep its help, a success and a refusal, and for a
  command that offers `--json`, a JSON success and a JSON refusal.
- The release journeys in `tools/release` drive a published baseline and the
  candidate through the same flows, each with its own binary.
- Client projections embed `aigw credential <client> <fingerprint>`, so that
  command line is part of every projected client file.

## Goals / Non-Goals

**Goals:**

- A script drives every command that reports a result with `--json`, reads
  one versioned document shape per command, and follows `next_action` without
  parsing prose.
- Each flag name has one meaning, and each command's client scope follows one
  rule.
- Every write can be previewed, and the exit status depends only on the
  outcome.

**Non-Goals:**

- Fewer commands. Merging the overlapping commands changes how people work
  every day, and nothing measured shows that the overlap confuses them.
- Changes to the configuration schema, on-disk formats, client projections or
  release assets.
- A schema language. The Go result types and the golden acceptance scripts
  own the contract.

## Decisions

### Approach

Three ways to converge were considered:

- **One contract, the same commands (chosen).** One selection vocabulary and
  one JSON document contract, `--json` and `--dry-run` wherever they apply,
  and the exit status fixes, with every command kept. It fixes what makes
  automation guess wrong, and people keep the commands they know.
- **The contract plus merged commands.** For example `test` into `check`,
  `models` into `catalog`, `add` into `setup`, and `rollback` into
  `config rollback`. About five fewer commands, but daily habits and every
  guide change, and nothing measured shows that the overlap confuses anyone.
  It can follow later on evidence.
- **Versioned shapes without renames.** `schema_version` and the missing modes
  only. Nothing breaks, but the meaning conflicts stay, and a script still
  has to learn each command's spelling.

The first three stages below are common to all three and break nothing. The
fourth stage carries every break, and it starts only after the product owner
approves the converged contract.

### Objects are positional, qualifiers are flags

The thing a command acts on is a positional argument: `use <route>`,
`route show <route>`, `account edit <account>`, `client enable <client>`. An
optional Account argument still defaults to the only Account. Every qualifier
is a flag, and each flag name has one meaning in every command:

| Flag               | Meaning                                         |
| ------------------ | ----------------------------------------------- |
| `--for`            | A client ID, or `all` for the enabled clients   |
| `--account`        | An Account ID                                   |
| `--route`          | A Route ID                                      |
| `--model`          | A canonical Model ID                            |
| `--upstream-model` | The model ID that the Account's endpoint serves |
| `--protocol`       | An endpoint protocol                            |
| `--target`         | A client's native configuration file            |
| `--path`           | The program file that AIGW installs or removes  |

Two flags change name. `setup --model` and `add --model` name the upstream
ID, so they become `--upstream-model`, and `--model` does not return to
those commands in this Change. `install --target` and `uninstall --target`
name the program file, so they become `--path`. Each old spelling stays for
one release as a hidden flag bound to the same value. Cobra's
`MarkDeprecated` prints its replacement on standard error.

Client scope: without `--for`, `check` and `test` cover the enabled clients,
so `test` stops probing disabled clients. `verify` starts
real client sessions that may use quota, so it still requires `--for <client>`
or `--for all`, and `--for all` still writes the verified checkpoint.

Alternative considered: make `client enable` and `client disable` take
`--for`. Rejected: the client is those commands' object, as the Route is
`use`'s.

### One JSON document contract

Every command that reports a result takes `--json` and writes exactly one
document to standard output. A command with `--json` never prompts: a Token
comes from `--token-stdin`, and a missing one is a refusal. The exceptions write a foreign format:
`completion` writes a shell script, `config export` a manifest,
`credential` the client's credential protocol, and the interactive wizard
writes nothing. The rules:

- `schema_version` is an integer. Each command's document has its own
  version. This Change publishes version 1 of each document, except that
  `installation` keeps its existing version 1.
- An object's own identifier is `id`. A reference to another object is named
  after its type: `account`, `route`, `client`, `model`, and the model ID
  that the endpoint serves is `upstream_model`. `use`'s `route_id` and
  `account_id` become `route` and `account`. Its `model`, which holds the
  upstream ID today, becomes `upstream_model`, and its `canonical_model_id`
  becomes `model`, as in `route list`. Setup's `selected_bindings` becomes
  `selections`, and sync's `targets` and repair's `projections` become one
  `projections` list of `{client, surface, target, action}`.
- A `target` is home-relative, `~/…`, as doctor reports it today. A target
  outside the home directory is omitted.
- Lists are `[]`, never `null`. States and kinds are snake_case. A value that
  echoes configuration keeps its configuration spelling, such as the
  authentication mode `account-token`.
- `next_action` is a complete `aigw` command that a script can run as it
  stands. When the next step needs a value that AIGW cannot choose, such as
  which Route a client should use, `next_action` is absent and `next_detail`
  shows the command with its placeholder. Other prose, such as "Restart Claude
  Code first", also goes in `next_detail`. Human output's Next section shows
  `next_action`, or `next_detail` when it is absent. Doctor's per-check `fix`
  becomes `next_action`, and a rename's external to-dos stay in `actions`.
- Every document carries `ok`. A command that completes and finds a problem
  writes its own document with `ok: false` and exits 1: `check` when a client
  is not ready, `doctor` with a failed check, a blocked preview, or
  `verify --for all` when one client fails. A command that cannot produce its
  document writes the failure envelope instead: `ok: false` and the `Problem`
  fields. Only the envelope has `error`, and it has its own `schema_version`.
- When the configuration could not be loaded, the envelope's `stage` names
  the step, as human errors do today: `configuration_read`,
  `configuration_parse` or `configuration_validate`. A parse failure adds
  `line` and `column`. `status`, `check` and `sync` fail with this envelope.
  Doctor reports the same stage and position in its failed `config` check and
  exits 1. Other failures carry no `stage` in this Change.

Owners: `internal/terminal` owns the envelope and `terminal.Document`, which
heads every document with `schema_version` and `ok`, and the continuation
fields. Each command package owns its document type and embeds the type that
its use case returns, so the use case's fields stay at the top level.
`installation` already carries `schema_version`, so its document adds only
`ok`: two embedded fields with one JSON name would both be dropped.
`commands_test.go` extends to two rules. Every result-reporting command must
offer `--json`. Every JSON document that its script prints must carry
`schema_version`, snake_case keys, no `null` list, and, when present, a
`next_action` that is an `aigw` command.

Alternatives considered:

- One global contract version. Rejected: a change to one command's document
  would bump every command.
- Both shapes behind a `--json-schema` flag for one release. Rejected: it
  doubles every JSON test, and a reader that never asks for the new shape is
  not warned.

### Every write previews

Every command that writes configuration, client files or the program takes
`--dry-run`: `setup`, `add`, `route add`, `route edit`, `route remove`,
`account edit`, `client enable`, `client disable`, `rollback`, `update`,
`install` and `uninstall`, beside the commands that already take it. A
preview reports the plan in human and JSON output with `dry_run: true`.

A preview reads what its plan needs, including stored Tokens where the plan
compares them, as the renames do today. It sends no authenticated request,
downloads nothing, runs no other program, and leaves no file, credential or
Keychain change behind. So `setup --dry-run` and `add --dry-run` report the
Account, Route and Client Bindings they would create without checking the
Token. `update --dry-run` reads the public release metadata and reports the
version that it would install, without downloading it. `use --dry-run` stays
free of credential reads.

`rotate` and `account diagnostics enable` and `disable` write only the
credential store from secret input, so they gain no preview. `test
--token-stdin` already checks a Token without storing it.

### Exit status follows the outcome

A command exits 0 when it succeeds and 1 when it fails or refuses, with or
without `--json`. Four results change:

- `catalog --json` on an empty home exits 1, as human output does.
- A blocked rename preview exits 1.
- Bare `aigw config` prints its help and exits 0, as the other command groups
  do.
- `aigw help <command>` shows that command's help instead of the help
  command's own page.

`status` still exits 0 without configuration, because it reports state. `check`
and `doctor` judge readiness, so they still exit 1.

### Compatibility and migration

Releases from `dev` continue while this Change is in progress. Its first
three stages only add: fields, commands' `--json`, previews and the new flag
spellings. A release cut between them changes nothing that a script reads or
passes today. Every break lands in the last stage, which ships as 0.5.0.

The 16 documents that exist today gain `schema_version` 1 in the first
stage, with their current fields. Version 2, the converged shape, comes with
0.5.0. The 20 new documents start at version 1 with the converged rules. The
0.5.0 CHANGELOG has a table of every renamed flag and JSON field and every
changed default, value or exit status, with its replacement. `use`'s `model`
changes value under the same key, so the table marks it, and its acceptance
script uses different canonical and upstream IDs so that a mix-up fails.
README.md, the team rollout guide and the terminal experience document change
in that stage. Deprecated flag spellings are removed in the release after
0.5.0.

The release journeys run the published baseline and the candidate. A
baseline step keeps the baseline's flags and reads its JSON shape. Only
candidate steps move to the new surface.

## Stages

Each stage is one Work Lane and one PR into `dev`. Its acceptance is its
proof, hosted graph and the real-client journeys. The release follows the
last stage.

1. **Contract fields.** `schema_version` 1 and `ok` in the 16 existing
   documents and the failure envelope, the failure `stage` with its position,
   and the contract test rule that every document carries both fields.
   Additive. `next_detail` comes with the `next_action` rule in stage 4,
   because it changes what an existing field holds.
2. **Coverage.** `--json` for the 20 result-reporting commands that lack it,
   and for `setup` without `--from`, following the converged rules. Additive.
3. **Previews.** `--dry-run` for every command that writes, and
   `--upstream-model` and `--path` beside the old spellings. Additive.
4. **Convergence.** Version 2 of the 16 existing documents, deprecation
   warnings on the old flag spellings, the client scope rule, the exit status
   changes, help, the documents and the CHANGELOG table. This stage is the
   break. It starts after the product owner approves the converged contract,
   and 0.5.0 follows it.

## Risks / Trade-offs

- Scripts that read today's JSON break once, at 0.5.0. `schema_version` lets
  them detect the shape, and the CHANGELOG table names each field's
  replacement.
- A stage that adds a field to 16 documents changes every golden script that
  prints them. The additive stages change expected output, not meaning, and
  each script's diff shows only the added fields.
- A deprecation warning on standard error can fail a script that treats any
  standard error as failure. That lasts one release, and the warning names
  the replacement.
- `test` without `--for` stops probing disabled clients. A script that relied
  on that passes `--for <client>`.
- More previews mean more code paths that must write nothing. Each preview
  runs against a private home in its acceptance script, and the script checks
  that every file is unchanged.

## Open Questions

- None that block the design. Whether `--for` should take several clients
  waits for a measured need.
