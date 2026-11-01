## Why

AIGW's commands grew one at a time, and the concepts they share now have
several spellings. Measured at the head of `semantic-architecture` stage 5:

- **One flag, several meanings.** `--model` is the upstream model ID in
  `setup` and `add`, but the canonical Model in `route add`. `--target` is the
  program's destination in `install`, the program to remove in `uninstall`,
  and a client's native configuration file in `client enable`.
- **"All clients" has three rules.** Without `--for`, `check` probes the
  enabled clients, `test` probes every client with a selected Route, disabled
  ones included, and `verify` refuses. `verify --for all` means the enabled
  clients.
- **Automation cannot rely on JSON.** 16 commands take `--json`. 20 commands
  that report a result do not, `verify` among them, and `setup` writes JSON
  only with `--from`. Only `installation` versions its document. One concept
  has several field names: `route_id` and `route`, `selections` and
  `selected_bindings`, `targets[].target` and `projections[].surface_id`.
  `next_action` is a command in some documents and prose in others, and
  doctor names its continuation `fix`.
- **Previews are partial.** Of the commands that write configuration, client
  files or the program, only `use`, `sync`, `repair`, the two renames,
  `config import` and `config migrate` take `--dry-run`.
- **Exit status depends on the output mode or the command.** `catalog --json`
  exits 0 on an empty home, where human output exits 1. A blocked rename
  preview exits 0. Bare `aigw config` exits 1, where bare `aigw account`
  exits 0.
- **One failure, three shapes.** An unreadable configuration is an error
  envelope with its position in `status` and `sync`, the state `invalid`
  without the failed stage in `check`, and a failed check in `doctor`.

Scripts and agents that drive AIGW have to learn each command on its own, and
a wrong guess can select the wrong Model or remove the wrong file.

## What Changes

**BREAKING.** One selection vocabulary and one JSON contract for every
command:

- **Selection.** The object that a command acts on is positional. Each
  qualifier is a flag with one name and one meaning: `--for` a client,
  `--account`, `--route`, `--model` a canonical Model, `--upstream-model` its
  wire ID, `--protocol`, `--target` a client's native configuration file and
  `--path` the program file. `setup --model` and `add --model` become
  `--upstream-model`. `install --target` and `uninstall --target` become
  `--path`. Without `--for`, a command that sends no inference request
  covers the enabled clients. `verify`, which may use quota, still requires
  `--for <client>` or `--for all`.
- **JSON contract.** Every command that reports a result takes `--json`.
  - Each document carries `schema_version` and `ok`.
  - An object's own identifier is `id`, and a reference to another object is
    named after its type: `account`, `route`, `client` or `model`.
  - Lists are `[]`, never `null`, and states and kinds are snake_case.
  - `next_action` is one `aigw` command or absent, and any prose goes in
    `next_detail`.
  - Failures keep the error envelope and name the stage that failed.
- **Previews.** Every command that writes configuration, client files or the
  program takes `--dry-run`, which reports the plan and writes nothing.
- **Exit status.** 0 for success and 1 for a failure or refusal, the same
  with and without `--json`. A blocked preview exits 1.
- **Help.** `aigw help <command>` shows that command's help.

The work lands in additive stages first, so releases from `dev` can continue,
and every break lands together in 0.5.0. A renamed flag keeps its old
spelling for one release after that, hidden and with a deprecation warning on
standard error. `schema_version` tells a reader which document shape it has.

**Unchanged:** the list of commands, the configuration schema and on-disk
formats, client projections, release assets, and the credential helper
command line that projections embed (`aigw credential <client> <fingerprint>`).

**Reuse stance:** Cobra's flag deprecation, the existing error envelope and
`Problem`, the command-tree contract test, and the golden acceptance scripts.
No dependency is added.

**Non-goals:** merging overlapping commands (`test` and `check`, `models` and
`catalog`, `add` and `setup`, the three rollbacks), renames in the
configuration schema such as `account_probe`, new commands, and the proxy
repository.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `command-line`: one flag vocabulary and client scope, the JSON document
  contract, `--dry-run` for every command that writes, exit status and help.
- `cli-readiness`: readiness documents follow the JSON contract, and check and
  doctor name the stage of an unreadable configuration.
- `program-lifecycle`: `install` and `uninstall` take `--path`, and `update`,
  `install` and `uninstall` preview with `--dry-run`.

## Impact

Command packages under `internal/cli/`, rendering in `internal/terminal`, the
acceptance scripts and their per-platform expectations, the release journeys'
candidate steps, README.md, the team rollout guide and the terminal
experience document. The release journeys keep driving the published
baseline with that release's flags and JSON. The change ships as 0.5.0, and
its CHANGELOG lists every rename with its replacement.
