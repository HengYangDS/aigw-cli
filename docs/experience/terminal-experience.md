---
subject: aigw:terminal-experience
role: explanation
state: canonical
relations:
  canonical_for: terminal interaction and recovery language
---

# Terminal Experience

AIGW human output answers three questions:

1. What is selected?
2. Is it ready?
3. What is the next safe action?

Readiness is decomposed rather than inferred. `status` reports each client's
model and whether its client files are ready, without reading Tokens or
invoking clients. `check` climbs one ladder of evidence: `--depth local`
checks without a request, `endpoint` sends one model-free authenticated
request, `model`, the default, sends one bounded inference request per checked
client and may use provider quota, and `client` runs a real client and may make
several requests. Synchronized client files alone are not proof of
authentication or inference. `check <client>` checks only the named client's
selection, credentials, files, and endpoint; it does not observe unrelated
clients or providers. `sync` changes only AIGW-owned configuration, never
client-owned credentials.

## Navigation

`aigw <command> --help` is the source-derived command reference. Cobra owns
command grammar, descriptions, examples and available subcommands; pflag owns
option notation, value types, declared defaults and inherited options. AIGW
adds journey grouping and terminal layout, not a second option-definition
table. Help is available before configuration and does not create local state.

Runnable commands show their invocation; command groups show `[command]`.
A command that supports both shows both forms. The root journey uses
`aigw use <client> <provider/model>`: the client, not the model, owns
selection. Interactive invocation may prompt for omitted values; automation
must state both. The command grammar remains the automation contract. Every
leaf command shows one to three examples, and shell completion offers
clients, providers, `provider/model` names, protocols and depths from local
configuration, without reading a Token or sending a request.

Argument admission precedes configuration locking and command execution. An
explicit empty or whitespace-only update path is invalid, not an online-update
request. Team-file setup validates its path, explicit provider IDs and option
combinations before creating configuration storage. Cobra remains the owner of
required flags and flag relationships; domain validation runs with the operation.
A removed command or flag is refused as unknown and names the help to read.

## Output model

Human and JSON views render the same observed result; neither view defines a
separate readiness model.

| Surface  | Contract                                                              |
| -------- | --------------------------------------------------------------------- |
| Human    | Task-first, aligned, width-aware, one safe next action                |
| JSON     | Stable machine fields; no terminal styling or width dependency        |
| Error    | **Problem → Evidence → Fix**, with an impact only when it adds a fact |
| Pipeline | Plain text, no ANSI control sequences                                 |

JSON output uses one UTF-8 document, two-space indentation and a trailing
newline. The presentation owner delegates encoding to Go's standard library;
command owners retain their schemas and exit-status decisions. Terminal width
and color do not alter machine output.

`provider list`, `model list`, and `status --json` expose deterministic,
secret-free JSON inventories; client-keyed state uses stable client
identifiers, and each model is named `provider/model`. A client that selects no
model remains explicit and includes its next usable `aigw use` step when one
exists.

For a command whose parsed `--json` value is true, a failure before any result
is written produces one JSON document with its own `schema_version`:
`ok: false`, `error`, available `evidence` and `impact`, and the next step. A
configuration that cannot be loaded adds its `stage`, and a parse failure its
`line` and `column`. `--json=false` keeps human output; a literal
`--json` after `--` is an argument, not a formatting flag. A usage error, such
as an unknown command or flag, is a JSON document wherever `--json` stands in
the arguments before a `--` that ends the flags; `--json=false` keeps text.

Commands that already render their diagnostic result retain that result and a
nonzero exit status. A partial JSON write is not completed with prose or a
second document. Any command, output, or cleanup error remains a failure;
successful error rendering cannot turn it into success. Credential-helper
stdout carries only the requested credential, never root-level diagnostics.

## Layout

- Each group of rows shares one value column set by its longest label; wrapped
  values and details continue under that column.
- When too little width remains for values, the whole group puts each label
  and its wrapped value on separate lines.
- Display width, not byte length, controls alignment.
- Words and paths are not split merely to fill a line, and identifiers keep
  their hyphens.
- Notes keep one indentation at every width.
- Clients and protocols appear under their labels, such as `Claude Desktop`
  and `OpenAI Responses`; help, flags and copyable commands keep the
  identifiers you type, such as `claude-desktop`.
- Color is optional and never carries meaning.
- Unknown output width uses a stable unbounded layout.

`COLUMNS` may provide an explicit positive width. JSON ignores it.

## Interaction

| Context                            | Behavior                                                |
| ---------------------------------- | ------------------------------------------------------- |
| Interactive, missing arguments     | Prompt only for missing values                          |
| Non-interactive, missing arguments | Fail immediately with an actionable message             |
| `--dry-run`                        | No mutation lock, credential write, or client execution |
| `--json`                           | Machine-readable output; execution behavior unchanged   |

## Troubleshooting journey

Use the least powerful command that answers the current question:

1. `aigw status` reports each client's model and the next useful action.
2. `aigw check --depth local --verbose` prints every local check: the
   configuration, each Token's presence, each client's files and conflicting
   `ANTHROPIC_*` or `OPENAI_API_KEY` variables, without a request.
3. `aigw check --depth endpoint` authenticates through one model-free request
   per checked client and incurs no inference call.
4. `aigw check` sends one capped inference request carrying each checked
   client's selected model.
5. `aigw check <client> --depth client` invokes the real client to prove its
   selected model path. This request may consume quota even when the `model`
   depth has already passed. When the session stops on a warning or an error
   that Codex or Claude Code itself reports, `--verbose` adds the client's own
   lines after AIGW's, without the provider's Token and the known credential
   forms. A session whose credential AIGW does not hold shows none.
6. `aigw sync --dry-run --json` previews only AIGW-owned reconciliation;
   `aigw sync` applies that bounded plan.

`--model <provider/model>` with one named client tries that model without
selecting it. With `--token-stdin` it uses a one-time Token at the `endpoint`
or `model` depth and never reads or writes the credential store; the Token is
the raw value, so a broker that reads a native store decodes its storage
envelope itself.

Without a named client, `check` covers the enabled clients; a named client is
checked even when it is disabled. Each row names the deepest depth that its
client reached, a client stops at its first failed depth, and the other
clients still run. The command exits 0 when no checked client failed and 1
otherwise, including when no client is enabled: an empty check is not health
evidence.

A depth that a client cannot run is skipped with its reason, and a skip is not
a failure. For client-native authentication, `check` checks the local client
files without accessing client credentials or calling the endpoint. If Claude
Code has changed only its native model and the AIGW sidecar still proves the
endpoint and helper, Claude has no `model` depth, because AIGW does not know
which model Claude Code sends; it never treats the native alias as the
provider's model ID. Claude Desktop has no `client` depth, because it needs
acceptance inside the app. `status` makes no request.

The JSON vocabulary follows that evidence boundary:

| Field or state            | Exact meaning                                                              |
| ------------------------- | -------------------------------------------------------------------------- |
| `depth`                   | The depth that `check` asked for, and the deepest that each client reached |
| `skipped`                 | Each depth that a client could not run, with its reason                    |
| `failure`                 | The depth at which a client failed, its problem and its HTTP status        |
| `failure.client_report`   | With `--verbose`: the client's own lines behind a failed session, redacted |
| `endpoint_configured`     | The selected model resolves an endpoint address.                           |
| `client_files_ready`      | Status: the client's files match its selection.                            |
| `client_files_deferred`   | Status: the client's files wait for the client or its Token.               |
| `native_model_preference` | Claude has a sidecar-proven native model preference.                       |
| `configured`              | Local prerequisites pass; no successful endpoint evidence.                 |
| `snapshot_written`        | `check --depth client` wrote the verified snapshot for a rollback.         |
| `ok`                      | The command's stated check scope passed.                                   |
| `enabled_clients`         | Number of enabled clients examined.                                        |
| `next_action`             | A next command that a script can run as it stands.                         |
| `next_detail`             | A step for a person: a placeholder, prose or a prompt.                     |

Neither `ok` nor the `model` depth proves real-client execution, future
availability, or the provider's retention policy. The `endpoint` depth does
not establish model inference.

`aigw check --depth client` with no named client writes the verified snapshot
that `aigw config rollback` restores, when no client failed and every client
that can run a session ran one. The snapshot records each client's depth.

For `sync`, `--json` selects output format, not execution mode. Add
`--dry-run` to preview without writes; omit it to apply. A preview contains
`dry_run: true`, and every result names its next step as `next_action` or
`next_detail`: previews with state changes point to the apply command, while a
converged active selection points to `aigw check`. An empty `sync` selection
instead reports deferred activation and a provider or client prerequisite. The
`client_files` list appears only in previews, each file relative to the home
directory; applied output does not present an earlier plan as observed
per-file completion. `sync` never changes credentials.

An output failure after a successful commit does not undo the write to the
client files.
Inspect current state before retrying; generic error rendering cannot promise
rollback. Only the transaction owner can report completed compensation.

Credential-entrypoint finalization follows that same recovery contract:
restored configuration does not prove that every client or reader was restored.
Program update and rollback distinguish completed replacement from failed
temporary cleanup; the replacement remains active even when cleanup fails.
Portable uninstall commits client withdrawal before removing program files.
If removal fails, that withdrawal remains committed. Without configuration,
no withdrawal is claimed. Resolve file access and retry from another verified
AIGW executable if the selected command has already been removed.

No recovery command edits conversation history, client-private databases,
Desktop-only settings, or an external compatibility service.

## Boundary language

A configured loopback address is described as a **loopback endpoint**, not an
inferred compatibility layer. Human status identifies every affected client;
JSON uses `external_loopback` for the same address observation. Neither view
establishes service identity, listener health, or lifecycle ownership.

Provider endpoints and readiness share the standard IPv4 and IPv6 loopback
classification, including IPv4-mapped addresses and case-insensitive
`localhost`. Classification performs no DNS lookup or service probe. Private
and unspecified network addresses are not loopback and still require HTTPS.

`aigw use` and `aigw client` manage a client's selection and AIGW's settings
in its files only. They do not inspect or control IDEs, external proxies,
desktop-only state, or conversations.
