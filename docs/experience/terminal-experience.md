# Terminal Experience

AIGW human output answers three questions:

1. What is selected?
2. Is it ready?
3. What is the next safe action?

Readiness is decomposed rather than inferred. `status` reports selection and
local projection readiness without reading Tokens or invoking clients.
`check` adds one bounded Route-model inference by default or an endpoint-only
check when requested; `verify --for <client>` runs a real client. A synchronized
projection alone is not proof of authentication or inference.
`sync` changes only AIGW-owned configuration, never client-owned credentials.

## Navigation

`aigw <command> --help` is the source-derived command reference. Cobra owns
command grammar, descriptions, examples and available subcommands; pflag owns
option notation, value types, declared defaults and inherited options. AIGW
adds journey grouping and terminal layout, not a second option-definition
table. Help is available before configuration and does not create local state.

Runnable commands show their invocation; command groups show `[command]`.
A command that supports both shows both forms. The root journey uses
`aigw use --for <client> <route>`: the client binding, not the reusable
Route, owns selection. Interactive invocation may prompt for omitted values;
automation must state both. The command grammar remains the automation contract.

Argument admission precedes configuration locking and command execution. An
explicit empty or whitespace-only update path is invalid, not an online-update
request. Manifest setup validates its path, explicit Account ID and option
combinations before creating configuration storage. Cobra remains the owner of
required flags and flag relationships; domain validation runs with the operation.

## Output model

Human and JSON views render the same observed result; neither view defines a
separate readiness model.

| Surface  | Contract                                                       |
| -------- | -------------------------------------------------------------- |
| Human    | Task-first, aligned, width-aware, one safe next action         |
| JSON     | Stable machine fields; no terminal styling or width dependency |
| Error    | **Problem → Evidence → Impact → Recommended action**           |
| Pipeline | Plain text, no ANSI control sequences                          |

JSON output uses one UTF-8 document, two-space indentation and a trailing
newline. The presentation owner delegates encoding to Go's standard library;
command owners retain their schemas and exit-status decisions. Terminal width
and color do not alter machine output.

`account list`, `route list`, and `status --json` expose deterministic,
secret-free JSON inventories. Account and Route IDs use lexical order;
client-keyed state uses stable client identifiers. Route inventory names credential
ownership as `aigw`, `external`, or `client`, and only AIGW-owned credentials
include availability metadata. Unselected Client Bindings remain explicit and
include their next usable `aigw use` action when one exists.

For a command whose parsed `--json` value is true, a failure before any result
is written produces one JSON document: `ok: false`, `error`, `next_action`, and
available `evidence` and `impact`. `--json=false` keeps human output; a literal
`--json` after `--` is an argument, not a formatting flag. Command-resolution
failures before flag parsing use human output.

Commands that already render their diagnostic result retain that result and a
nonzero exit status. A partial JSON write is not completed with prose or a
second document. Any command, output, or cleanup error remains a failure;
successful error rendering cannot turn it into success. Credential-helper
stdout carries only the requested credential, never root-level diagnostics.

## Layout

- Wide terminals use one aligned value column.
- Narrow terminals put the label and wrapped value on separate lines.
- Display width, not byte length, controls alignment.
- Words and paths are not split merely to fill a line.
- Color is optional and never carries meaning.
- Unknown output width uses a stable unbounded layout.

`COLUMNS` may provide an explicit positive width. JSON ignores it.

## Interaction

| Context                               | Behavior                                                |
| ------------------------------------- | ------------------------------------------------------- |
| Interactive, missing rename arguments | Prompt only for missing values                          |
| Non-interactive, missing arguments    | Fail immediately with an actionable message             |
| `--dry-run`                           | No mutation lock, credential write, or client execution |
| `--json`                              | Machine-readable output; execution behavior unchanged   |

## Troubleshooting journey

Use the least powerful command that answers the current question:

1. `aigw status` reports selected Client Bindings and the next useful action.
2. `aigw check` validates configuration, credentials, and client projections,
   then makes one capped inference request carrying each eligible selected
   Route's exact upstream model. `--endpoint-only` instead authenticates through
   a model-free catalogue request and incurs no inference call.
3. `aigw doctor` explains structural or host integration failures without
   mutation.
4. `aigw repair --dry-run --json` previews only AIGW-owned reconciliation;
   `aigw repair` applies that bounded plan.
5. `aigw test` tests selected Account endpoints. `--for <client>` chooses the
   client; optional `--route <route>` overrides that client's current
   binding for this read-only request. With no selected binding it fails and
   recommends `aigw use --for <client> <route>` rather than reporting an empty
   success. A one-time `--token-stdin` request requires an explicit client and
   never accesses the credential store; optional `--config` selects an absolute
   configuration file. Its result is HTTP observation, not model inference or
   native-client proof.
6. `aigw verify` invokes the real native client to prove its selected model
   path. This request may consume quota even when `check` has already passed.

`check` exits successfully when enabled Client Bindings pass the applicable
scope. With no enabled client, success covers configuration only. For
client-native authentication it checks the local projection without accessing
client credentials or calling the endpoint. A selected Account-Token Route
normally receives one model-carrying inference diagnostic; the default request
is metered and establishes only that time-bound request's acceptance.
`--endpoint-only` preserves the cheaper model-free endpoint scope. If Claude
Code has changed only its native model and the AIGW sidecar still proves the
endpoint and helper, `check` uses endpoint scope for Claude and recommends
`aigw verify --for claude`; it never treats the native alias as the Route's
upstream wire model. Status and doctor make no inference request.

The JSON vocabulary follows that evidence boundary:

| Field or state          | Exact meaning                                                   |
| ----------------------- | --------------------------------------------------------------- |
| `endpoint_configured`   | The selected Route resolves an endpoint address.                |
| `projection_ready`      | The local client projection passes inspection.                  |
| `native_model_override` | Claude has a sidecar-proven native model preference.             |
| `diagnostic_scope`      | `endpoint` or `inference`, whichever request was performed.     |
| `check_passed`          | The binding passed the checks applicable to its scope.          |
| `configured`            | Local prerequisites pass; no successful endpoint evidence.      |
| `endpoint_checked`      | A model-free authenticated endpoint request succeeded.          |
| `inference_checked`     | One exact Route-model inference request succeeded.              |
| `ok`                    | The command's applicable checks passed.                         |
| `next_action`           | The next explicit action, not necessarily a repair.             |

Neither `ok` nor `inference_checked` proves real-client execution, future
availability, or the provider's retention policy. `endpoint_checked` does not
establish model inference.

`verify --for all` means all enabled client bindings. It checks their local
prerequisites before invoking any client, then writes a checkpoint only after
all requested verifications succeed. Disabled clients are not prerequisites;
no enabled clients produces an explicit error, not an empty success. Account
rename finalization accepts that current scope. Without enabled clients, it
checks credential and backup continuity without requiring an inference proof.

`doctor` uses the same outcome for human and JSON output. Failed diagnostics or
invalid, degraded, or unavailable client observations produce a nonzero exit
status and `ok: false`. Deferred clients are not failures by themselves. A JSON
report remains one document on failure, with `next_action` carrying the same
continuation shown in human output. Writing the report successfully does not
make a failed diagnosis successful.

For `sync` and `repair`, `--json` selects output format, not execution mode.
Add `--dry-run` to preview without writes; omit it to apply. A successful
result contains `dry_run` and `next_action`: previews point to the apply
command, while applied results point to `aigw check`. Projection plans appear
only in previews; applied output does not present an earlier plan as observed
per-target completion. Neither operation changes credentials.

An output failure after a successful commit does not undo the projection.
Inspect current state before retrying; generic error rendering cannot promise
rollback. Only the transaction owner can report completed compensation.

No recovery command edits conversation history, client-private databases,
Desktop-only settings, or an external compatibility service.

## Boundary language

A configured loopback address is described as a **loopback endpoint**, not an
inferred compatibility layer. Human status identifies every affected client;
JSON uses `external_loopback` for the same address observation. Neither view
establishes service identity, listener health, or lifecycle ownership.

Account admission and readiness share the standard IPv4 and IPv6 loopback
classification, including IPv4-mapped addresses and case-insensitive
`localhost`. Classification performs no DNS lookup or service probe. Private
and unspecified network addresses are not loopback and still require HTTPS.

Client Binding commands manage AIGW Route selection only. They do not inspect
or control IDEs, external proxies, desktop-only state, or conversations.
