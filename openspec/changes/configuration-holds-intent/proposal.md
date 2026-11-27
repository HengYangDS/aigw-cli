## Why

`aigw.toml` mixes what the operator chose with what AIGW observed. Its `executable` key is written by discovery as
if the operator had chosen it, so a versioned editor-extension path becomes the client's identity and dies with the
next extension upgrade (GitLab issue 1; the stale loop that followed is already fixed by Change
`missing-program-reported-truthfully`). Its `authentication` and `model_provider` keys describe a Codex client-native
path that no command creates and no real client journey has proved; with retries set it writes a model-provider
table that Codex 0.172 rejected in an end-state committee member's experiment.

On 2026-11-25 at 17:28 the owner chose the end state: after the Windows removal and Change A, one schema 9 Change
that renames `executable` to `program` with an editor extensions directory as the alternative, removes the
client-native path with `authentication` and `model_provider`, and records the end-state rules as a decision record.
The design rests on the issue-1 committee and the end-state committee, whose material is kept under
`build/evidence/issue-1-committee-20261125/` and `build/evidence/end-state-design-20261125/` of the main checkout.

## What Changes

- **Breaking:** `aigw.toml` and team files move to `version = 9`. This release refuses a version 8 file before reading
  anything else and names the edits that schema 9 needs; it never rewrites the file itself. A release before this one
  refuses a version 9 file, so a program rollback to it needs the copy of `aigw.toml` kept before the edit.
- **Breaking:** a client declares its program by `program` (an absolute path) or by `extensions` (an editor's
  extensions directory), one of the two or neither. AIGW reads the declaration and never rewrites it. For
  `extensions`, each operation takes the newest installed version of the client's extension for this host, ignoring
  versions that the editor lists as obsolete, and stops the client when that version holds no program. A `program`
  inside a version of an editor extension is refused with the `extensions` declaration that names its directory. A client with no declaration adopts the program that
  it finds on the PATH, outside any editor extension, or at the client's fixed application path, once, and writes it
  as `program`, saying so.
- **Breaking:** `aigw client enable` and `aigw client edit` take `--program <path>` or `--extensions <dir>`, which
  exclude each other; `--executable` is gone. A versioned path inside an extensions directory given to `--program` is
  refused with the `--extensions` command that declares it instead. Every repair that named `--executable` names
  `--program`.
- **Breaking:** the client-native path is removed: the client keys `authentication` and `model_provider` and the same
  keys of a recommendation leave the schema, and Codex receives only AIGW's own provider table with the credential
  command. Nothing replaces them now; cloud providers return under the rules of DR-0013 when a real journey needs them.
- **Breaking:** a client's forwarding names the forwarding service's root (`aigw client edit <client> --via <root>`),
  and AIGW composes the destination from the selected provider's own Responses endpoint as
  `<root>/<scheme>/<authority>/<base path>`, the grammar of openai-responses-proxy 8.0.0, which refuses the
  provider-named routes. Forwarding stays when the client changes provider; a provider whose endpoint the grammar
  cannot name is refused for a forwarded client. A client's `forwarding` becomes one string, the root.
- **Breaking (output):** `aigw client list` JSON replaces `executable` with `program` and adds `extensions` where one
  is declared; its human output names the declared extensions directory beside the program it resolved.
- Whether a client's files are projected follows from its enabled state, its targets and AIGW's record, not from a
  non-empty program path.
- `aigw.toml` keeps its name: the credential reader never read `program`, and every reader since 0.7.0 reads an
  absent `authentication` as the provider-Token kind, so running sessions keep their Token. This release's reader no
  longer decodes `authentication`.
- DR-0013 records the end-state rules; DR-0012 point 4 gains the sentence that explains why removing
  `authentication` needs no new file name.

## Capabilities

### New Capabilities

- `client-programs`: which program each client uses: its declaration, its resolution, adoption, the client flags and
  the stop when it is missing.

### Modified Capabilities

- `product-control-plane`: schema 9 and the refusal of a version 8 file; its requirement that every client selects a
  compatible model moves to `model-selection`.
- `client-adapters`: the client-native Codex provider is removed; its program requirements move to `client-programs`.
- `credential-continuity`: the frozen view across schema 9.
- `progressive-team-onboarding` and `shipped-team-file`: team files at version 9 without client-native keys.
- `model-selection`: the client list's program fields; forwarding through a service root; it takes the
  compatible-model requirement.
- `client-chosen-model`: a followed model ends a forwarding only when the grammar cannot name its upstream, and an
  explicit selection of another upstream keeps the forwarding.
- `projection-format`: Codex's provider table without a client-native form.
- `secret-storage` and `client-verification`: repairs that name `--program`.
- `secret-storage` and `cli-readiness`: the client-native case leaves, and an own `credential_command` takes its place.
- `command-line`: the argument checks of `--program` and `--extensions`.

## Impact

`internal/domain` (configuration version, client binding, client specification, selection validation),
`internal/store` (schema 9 documents), `internal/adapter` (program resolution, discovery, the Codex provider table, the
four adapters' program use), `internal/cli/client` (flags, list), `internal/app` (activation, status, check),
`manifests/team.toml`, `tools/release` journeys, their tests and golden files, `docs/` (configuration and client
references, DR-0012, DR-0013), `CHANGELOG.md`.

Out of scope: cloud providers, identity references and any cloud account use; reporting other installations that may
read the same Codex home; a built-in list of editor extension directories; reading an editor's `extensions.json`;
running an editor's CLI; a generic adapter engine; new clients (pi, opencode); the real Remote SSH acceptance.
