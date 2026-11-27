---
subject: aigw:team-rollout
role: how-to
state: canonical
relations:
  canonical_for: team setup and rollout
---

# Team Rollout

A team distributes a reviewed public team file; each member supplies Tokens
locally. Applying the file with `aigw setup --from` requires neither Tokens nor
installed clients; activate each client when its prerequisites are available.

## Maintainer

1. Download the reviewed token-free
   [`manifests/team.toml`](../../manifests/team.toml).
2. Add only reviewed provider endpoints and qualified models.
3. Keep Tokens, personal paths, identities, and release credentials out.
4. Validate the manifest in a clean repository environment.
5. Publish it through the team's ordinary configuration channel.

Distribute the reviewed file from a release tag or immutable commit in either
Forge, not a moving branch. Open `manifests/team.toml` at that revision and use
the Forge's raw-file download; save it as `team.toml`. Do not save the rendered
HTML page. The portable program archive does not include this team-specific
configuration. Keep the selected revision with the team's rollout instructions
so members receive the same public configuration without cloning the repository.

A team file should contain the minimum set of models that users need. Provider
catalogs are discovery input, not automatic routing policy. Teams own model
choice; AIGW does not infer capability, quality or version policy from a model
ID. Adding a compatible model to an existing provider and supported client
changes configuration, not the Adapter implementation. New client or authentication
boundaries follow [Adapter admission](../governance/adapter-admission.md).

### Team file authoring contract

The [team file reference](team-file.md) defines the file's fields, its labels and layout, how to check a
provider's catalog, and what admits, deprecates and retires a model.

### Reviewed model defaults

The tracked [team file](../../manifests/team.toml) is the current model
inventory. It keeps the newest qualified model of every Claude line (Fable,
Opus, Sonnet, Haiku), GPT line (Astra, Sol, Terra, Luna) and Gemini line
(Carbon, Argon, Hydrogen), plus one reviewed general family per other
qualified vendor. A newer model of a line replaces the older one.
The [dated qualification
evidence](../research/provider-model-qualification.md) explains the observed
IDs, protocol tests, exclusions, and limits. A catalog listing is not a live
model or native-client availability guarantee.

Each client recommends one model on every provider that serves its family, in
the file's provider order: Claude Code and Claude Desktop Opus 5.5, Codex and
Hermes GPT-6.2 Sol (Codex DMXAPI's CDX and Suhe.AI's `codex-` model). A Token of
any one provider configures every client. A shared family does not make every
provider and client pair interchangeable. Display order never creates automatic
recovery when the selected provider fails.

A local provider endpoint override is not silently replaced by the team's
direct endpoint. Applying the team file again with `aigw setup --from` keeps
the local endpoint and reports the difference; change it deliberately with
`aigw provider edit <provider> --endpoint <protocol>=<url>` only when adopting
the team's direct endpoint.

Codex can run a compatible Responses model even when the model is absent
from its native `/model` chooser. Select it with
[`aigw use codex`](../../README.md#use-it-every-day), then verify through
Codex; the [dated client observation](../research/provider-model-qualification.md#codex-native-chooser)
does not establish current upstream availability. For an AIGW-managed custom
Provider, AIGW uses the installed Codex client's complete native catalog when
no exact entry can be produced for the selected model. It never invents
an entry for an unknown model or treats a provider `/models` response as
Codex-native metadata.
An explicit user-owned `model_catalog_json` may mask newer models bundled with
Codex. AIGW preserves that setting; if Codex reports fallback model metadata,
review the catalog before deliberately updating or removing it.

Recommendations apply only to unselected clients at setup or sync; they do not
switch a running request or replace a client's explicit selection. AIHubMix uses the
[documented API endpoint](https://docs.aihubmix.com/en/quick-start),
`api.inferera.com`: `/v1` is the Responses and Chat Completions base path; the
Anthropic base is the domain root; Suhe.AI's `suheai.cn` is laid out the same way, and its
[prefixed models](../research/provider-model-qualification.md#november-19-suheai) are priced apart. A successful
minimal request remains narrower than full real-client tool, continuation, streaming, and long-context acceptance.

Setup selects only a usable declared primary model or ordered alternative.
Other models remain available for explicit use, not automatic fallback by
family or identifier. No provider Token is mandatory, and a team file
preserves each client's existing selection.

Reasoning effort remains a native client preference, outside the team file's
version 8 schema. Choose a level supported by the selected model through Codex's
`model_reasoning_effort`, Claude Code's `effortLevel` or Hermes's
`reasoning_effort` in their active settings. Merge those fields; do not replace
any document. Applying the team file does not set these preferences.

For daily work the team uses each vendor's main line, the line below its
flagship. GPT-6.2 Sol and Claude Opus 5.5 run at `xhigh`. Gemini 4 Argon has
three levels, and its top, `high`, is the closest to `xhigh`, so Argon runs at
`high`. Every provider of the file serves these lines, so the same choice holds
for DMXAPI, UCloud and AIHubMix. Set the level explicitly: the defaults differ
by layer. Anthropic's API runs Opus 5.5 at `medium` when a request names no
level; OpenAI recommends `medium` to start with GPT-6.2; and Codex's bundled
catalog makes `low` the default of GPT-6.2 Sol and Astra.

| Vendor    | Flagship | Daily main line | Daily effort | API effort levels                 |
| --------- | -------- | --------------- | ------------ | --------------------------------- |
| OpenAI    | Astra    | GPT-6.2 Sol     | `xhigh`      | `low` to `max`                    |
| Anthropic | Fable    | Claude Opus 5.5 | `xhigh`      | `low` to `max`                    |
| Google    | Carbon   | Gemini 4 Argon  | `high`       | `low`, `medium`, `high` (its top) |

Every other vendor keeps one family in the team file, its strongest generally
available model; use it at the level below its top, or at its only level.
Whether a provider passes an effort level through to the model unchanged is not
established: the team's qualification runs Hermes at `medium`.

AIGW tells Hermes the context window of each model that it lists, and Codex
the selected model's: 1,000,000 tokens unless the model declares another
`context_window`. Claude Code and Claude
Desktop take a model's window from the Anthropic model that the provider's ID
serves. Compaction stays a client setting. A declared window, a catalog listing
or a successful short request does not prove a full-window request will
succeed.

- **Codex:** the model table that AIGW writes gives the selected model's entry
  the declared window as `context_window` and `max_context_window`. Codex
  counts 95% of that window as usable and starts compaction at 90% of it.
  `model_context_window` in `config.toml` can lower the window and cannot raise
  it above the entry's; `model_auto_compact_token_limit` can lower the
  compaction threshold. A model whose family Codex's own table lacks has no
  entry, and Codex chooses its window itself whatever the model declares; for a
  `model_catalog_json` of your own, AIGW writes no table and Codex is not given
  the declared window. A check of
  `aigw check codex` says which applies: `aigw check codex --verbose` lists it,
  and `--json` always holds it. The optional
  `model_auto_compact_token_limit_scope` selects `total` (the default, counting
  the full active context) or `body_after_prefix` (growth after the carried
  compaction-window prefix). Changing the counting scope does not enlarge the
  model's window. See the [Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).
- **Hermes:** each model that AIGW lists under a provider carries its declared
  window as `context_length`. When Hermes starts compression inside that
  window is its own `compression` setting, which AIGW does not write. For a
  model whose window differs from the number that Hermes knows, Hermes's log
  may warn that `model.context_length` pins it and that removing
  `model.context_length` from `config.yaml` gives the advertised window. The
  number is AIGW's per-model `context_length`, and the pin wins as intended.
  Change it with `aigw model edit --context-window`: AIGW refuses an edit of
  its entries in Hermes's file, a removed key or reordered models included, as
  an outside edit.
- **Claude Code:** [`autoCompactWindow`](https://code.claude.com/docs/en/settings-reference#autocompactwindow)
  sets the automatic compaction window, not provider capacity. Claude Code caps
  it at the model's window. Leaving it unset uses the client's model-specific
  default; `CLAUDE_CODE_AUTO_COMPACT_WINDOW` overrides `--autocompact`, which
  overrides the setting.

The native client lifecycle acceptance preserves these user-owned settings
through setup, synchronization, upgrade, rollback and uninstall. Its controlled
upstream requires the selected `high` effort in actual Codex and Claude
requests. Preserving context and compaction values does not establish a
provider's maximum context capacity or prove that compaction has executed.

### Client compatibility

Check `codex --version`, `claude --version`, or `hermes --version` for the
installed CLI, then run `aigw check <client> --depth client` for each enabled
client before rollout. The `client` depth runs Codex in a private home with your settings
but without your MCP servers, plugins, hooks, memories and notifications, so
those extensions neither start nor decide the result. Claude Desktop needs
separate application-version and native verification evidence.

A provider may
require a newer client even when an API request works. Update through the
existing installation owner rather than adding a second executable. Claude Code's
[`stable` and `latest` channels](https://code.claude.com/docs/en/setup#update-claude-code)
are distinct; choose deliberately when compatibility requires a channel change.
A passing request for one model does not qualify another model; verify the
selected model with the actual client version that team members will use.

If a gateway rejects `context_management` with `Extra inputs are not permitted`,
set `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1` in Claude Code's native settings
`env` and verify again. This [official compatibility
control](https://code.claude.com/docs/en/llm-gateway-protocol#disable-pre-release-capabilities)
removes pre-release request fields and beta tool-schema fields, including MCP
tool search and structured-output formatting. Ordinary tools, effort, and
supported extended-context headers remain. AIGW preserves this native preference;
it does not silently disable capabilities or edit the client. The isolated
`aigw check --depth client` invocation supplies a nonempty system prompt in
`--bare` mode.

A successful short request proves only that client, model and invocation.
It does not establish Desktop behavior, other operating systems, full-window
capacity, tool replay, or installation and update correctness. Verify those
journeys separately when the rollout depends on them.

## Upgrade from 0.6.1 or earlier

A member on 0.6.1 or earlier cannot read a version 8 team file: that release
refuses it as an unsupported manifest version, so everyone upgrades first. A
team file or an `aigw config export` in which a model declares
`context_window`, as the shipped file does, also needs the release that
introduced the field or later. It is still version 8, and 0.10.0 refuses it with
"schema 8 does not define the field providers.models.context_window". Then
each member:

1. Reads that 0.7.0 uses only `aigw.toml` and never reads or changes AIGW's
   `config.toml` or its component files, so the upgraded home starts without a
   configuration and `aigw` names `aigw setup`.
2. Runs `aigw setup --from team.toml`, or `aigw setup`.
   A provider added under its earlier ID finds the Token already stored for it,
   and setup uses that Token without a prompt.
3. Runs `aigw use <client> <provider/model>` for each client. A team file has no
   `[clients]` section, so a member sets forwarding and retries again with
   `aigw client edit`.

Client sessions that started before the upgrade keep working. Members delete
`config.toml` and `config.*.toml` once those sessions have ended. Hermes
sessions that started before the upgrade name a provider ID that no longer
exists on the responses or chat protocol, because those IDs are now
`aigw-<provider>-<protocol>`. `aigw self rollback`
cannot return to a release older than 0.7.0.

## New member

First follow [installation and command discovery](../../README.md#install).
The examples below use `aigw` after its installed directory is on `PATH`; the
installed executable's explicit path works with the same arguments. Applying a
team file does not install the program or change shell discovery.

Apply the reviewed team file without requiring every provider Token or any
installed client. Save the maintainer's `team.toml` in the current directory,
or pass its actual path to `--from`:

```bash
aigw setup --from team.toml
```

Setup:

- validates all public metadata first;
- keeps every reviewed provider and model;
- in a terminal, asks which providers without a stored Token the user holds
  one for, none allowed, and connects those that the providers accept;
- applies the file whatever becomes of each Token, reporting each Token that it
  did not store with its reason and next step;
- configures only installed supported clients;
- compensates failed client-file writes only where its owned writes remain
  unchanged, preserving newer external edits and reporting recovery conflicts.

To connect one provider during first-time setup, use this form **instead of**
the command above. The rest remain optional:

```bash
aigw setup --from team.toml --provider dmxapi
aigw status
```

The interactive command prompts only for the named provider's Token with a
writable backend. In read-only `env` mode, set `AIGW_TOKEN_DMXAPI` in the
invoking process; setup cannot store a Token there. With a writable backend,
automation may pipe exactly one Token by adding `--token-stdin`; it must keep
`--provider` so the Token's provider is explicit. The input is read through EOF
and is limited to 64 KiB, including an optional terminal LF or CRLF. A Token
contains only visible ASCII characters; embedded whitespace, extra lines and
control characters fail before validation or storage. A piped Token that its
provider does not accept leaves the rest of the team file applied and a Token
that the provider held in place, and the command exits 1.

For a one-time endpoint check,
`aigw check <client> --model <provider/model> --depth endpoint --token-stdin`
consumes that Token without reading or writing the credential store. One named
client and one model are required, so input cannot be reused across unrelated
providers. The `endpoint` depth reports an authenticated request that names no
model; the `model` depth adds one bounded inference request.

The Token is the raw value. A broker that reads the native macOS bytes written
by the pinned `go-keyring` backend decodes that storage envelope itself before
it pipes the Token: AIGW does not infer a format, decode an envelope, or
rewrite Keychain items.

If the team file is already applied, do not repeat setup. With the
environment backend, supplying one compatible provider variable and running
`aigw sync` selects unselected reviewed recommendations and writes the files of
available clients. With a writable backend, store the Token and select a model
explicitly:

```bash
aigw provider rotate dmxapi
aigw use codex dmxapi/gpt-6.2-sol-cdx
aigw status
```

One connected provider is enough to begin. Providers without Tokens remain
available but do not make another provider fail. With no enabled client,
`check` exits 1 without probing a provider; `aigw check --depth local` reports
zero enabled clients and does not claim client or inference readiness.
Neither requires Tokens for unselected recommended models.

If none is connected yet, setup, status, check, and sync preview present the
same choice of compatible providers. A writable credential store names each
`aigw provider rotate <provider>` command; the environment backend names each
variable. With a writable store, rotation only writes the Token; follow its
explicit `aigw use` step to point a client at a model. With the environment
backend, set a compatible variable before synchronizing. A team file alone
does not make `aigw sync` useful work.

When a model is selected but its client files are deferred, setup, `use`, sync
preview, status, and check retain that selection. If its provider's Token is
available, install the client if needed, then run `aigw sync`. If the Token has
disappeared, restore that provider's Token first; the missing Token and the
deferred client files remain distinct in machine-readable status. `check` stops
each client at the missing Token, which comes before the client files that wait
for it.

Interactive `aigw use <client> <provider/model>` can also prompt for that
provider's missing Token. Interactive use may prompt for the client or model;
non-interactive use requires both explicitly. If the Token is already available
and the selection is unchanged, selection performs no writes. A cancelled or
failed selection compensates its own credential writes; it preserves a newer
credential and reports any incomplete recovery. An output error after commit
does not undo the selection. Run `aigw status` before retrying.

### Hermes protocol and model selection

With Hermes installed and a connected DMXAPI provider using the team's direct
Responses endpoint, select and check its GPT-6.2 Sol model without changing
Codex or Claude selections:

```bash
aigw use hermes dmxapi/gpt-6.2-sol
aigw check hermes --depth client
```

If this machine explicitly points the DMXAPI provider at an external Proxy,
qualify that endpoint separately before selecting this model; applying the
team file does not silently replace the local endpoint.

Hermes's files include compatible models from connected providers; selecting
one active model does not remove the others. A model may be usable on more than
one protocol that Hermes speaks. If no previous selection or team
recommendation resolves that choice, interactive `use` asks which protocol to
use; non-interactive use requires one explicitly:

```bash
aigw use hermes <provider/model> --protocol responses
```

The choice is `anthropic`, `responses` or `chat`; AIGW never guesses from a
model name. A script that adds a provider for Hermes's chat protocol names
`aigw provider add <provider> --endpoint chat=<url> --token-stdin`, and the
protocol is stored in the client's selection. Guided setup and interactive
selection validate the selected protocol and endpoint before activation.
Team-file setup validates each Token that it reads with one request that names
no model, and uses a stored Token without a request. A provider whose model
catalog answers any Token, as AIHubMix's did on November 19, 2026, cannot
refuse a wrong one there, and no validation proves inference. `aigw check`
sends an inference request for each client's selected model; for a provider
that no client selects, run `aigw check <client> --model <provider/model>`.

Rotation validates and replaces only the named provider's Token. It does not
select a model, rewrite client configuration or invoke a native client. The
credential helper reads the new Token when next invoked; client caching may
require a reload. Use `aigw sync` for configuration changes. Failed storage
updates use guarded compensation, preserving newer Tokens rather than
overwriting them.

## Install a client later

Claude Code, Claude Desktop, Codex, and Hermes are not setup prerequisites.
After installing a selected client, run `aigw sync`; AIGW rediscovers supported
clients and converges only its owned configuration. Claude Desktop uses
`Claude-3p/configLibrary`, independently of Claude Code's `~/.claude` settings.
If a client has a selected model and usable authentication, sync enables it
and writes only AIGW's settings in its files. A client that uses a provider's
Token receives AIGW's credential helper, or the `credential_command` that you
named for it:

```bash
aigw sync
aigw check
aigw check <client> --depth client
```

Each client uses the program that it declares: `program`, a path that AIGW
adopted once from the `PATH` or that you named with
`aigw client edit <client> --program <path>`, or `extensions`, an editor's
extensions directory that you named with `--extensions <dir>`, where each
command takes the newest installed version of the client's extension for this
host. AIGW never rewrites the declaration and never swaps in another program.
Declare the extensions directory for Codex or Claude Code installed only as an
editor extension, as on a remote host of VS Code: a versioned folder inside it
disappears at the extension's next upgrade, so `--program` refuses one and
names the `--extensions` command instead. When the declared program goes,
`aigw sync` leaves that client's files as they are, synchronizes the other
clients and exits 1; `aigw client list` shows the declaration, and the repair
names it again.

`aigw status` observes selections and client-file readiness without client
execution or Token reads. `aigw check` adds one bounded inference request per
checked client by default; `--depth endpoint` keeps the model-free
authentication check, and `--depth client` runs the real client. Configuration
success alone is not authentication or inference proof.

Optional balance credentials do not take part in `aigw check`, in either human
or JSON output. Use
`aigw provider edit <provider> --balance-token-stdin --balance-user-id <id>` to
store them, which prompts with echo off in a terminal, and
`aigw provider balance <provider>` to request the provider's balance. An
unavailable balance service does not make a working client unhealthy. For
unattended setup, pipe the balance token line to the same edit; it is separate
from the provider's API Token, and the two flags go together, refused before
standard input is read. `--no-balance-token` removes them.

Claude Code, Hermes, and Codex clients that use a provider's Token call
credential helpers that match their client files. Changing the provider or
endpoint invalidates a retained helper invocation: run `aigw sync` and reload
the client's configuration. The helper does not return another provider's
Token to a client retaining the old endpoint.

## Existing member

Export local public metadata for comparison, then preview the reviewed team
file before applying it:

```bash
aigw config export > local-team.toml
aigw setup --from team.toml --dry-run --json
aigw setup --from team.toml
```

The preview applies the same validation as the write. It names the client
files that would change, not files that have been written or verified, and it
reads no Token and writes no configuration or client file.

For models, families and recommendations, what the file names wins, and
the local-only models that no client selects and no recommendation names are
removed, and then the families that no remaining model uses; `--keep-local`
keeps every local-only entry instead.
A team file never changes an existing provider's endpoints or Token and never
replaces a client's selection:

| Difference                              | Behavior                                                           |
| --------------------------------------- | ------------------------------------------------------------------ |
| A provider that only the file has       | Added; a Token already stored under its ID is used                 |
| An existing provider's label            | The file's label wins                                              |
| An existing provider's endpoint         | The local endpoint stays and is reported                           |
| A balance probe that the provider lacks | Added from the file; a differing probe stays local and is reported |
| A model, family or recommendation       | The file's version wins                                            |
| An unused local-only model or family    | Removed on import; `--keep-local` keeps it                         |
| A client's selection or a stored Token  | Never changed                                                      |

If this machine intentionally points DMXAPI at a local optional proxy, that
endpoint stays local while the file's models apply, and the result reports the
difference. `aigw provider edit` changes an endpoint deliberately. A file change
that would take a protocol from a client's selection keeps the local model and
is reported. A team file with a `[clients]` section, or in another version, is
refused before any write.

If writing a client file fails after a configuration write, AIGW names the
file and its operating-system error, says which client files it restored and
whether it restored its configuration, and names the fix: making the file and
its folder writable after a permission error, and `aigw check` otherwise.

Later `sync` may select an unselected client's reviewed recommendation when its
provider's Token appears in the read-only environment backend; it does not
search unused native credentials. An existing selection is preserved even if
its Token is unavailable, and a manual-only model is never selected
automatically. Use `aigw use <client> <provider/model>` to change a selection
explicitly.

## Local choices

Do not edit the downloaded team file to encode a personal default, local client
path, or workstation-only endpoint. Apply it as reviewed, then keep local
intent in AIGW's own configuration commands:

```bash
aigw use <client> <provider/model>
aigw provider edit <provider> --endpoint responses=<url>
aigw model add <provider/model> --protocol <protocol>
```

A model's name is `provider/model`, where the part after the first `/` is the
provider's exact model ID, including case or a channel suffix; AIGW does not
infer aliases. `--family` links a variant to its family, and
`aigw model edit` changes a model's protocols, family, capabilities, label or
context window.

`aigw use` changes only the named client. Provider and model commands change
local configuration and are not written back into the
[distributed team file](../../manifests/team.toml). To publish a team change,
review the team file itself and distribute the new token-free revision.

## Release installation

GitLab and GitHub are independent release sources. Verify one complete artifact
set from one source; do not mix a tag, checksum file, and archive across Forges.

| Platform | Install asset                                 |
| -------- | --------------------------------------------- |
| macOS    | Matching Darwin archive and checksum manifest |
| Linux    | Matching Linux archive and checksum manifest  |

After installation:

```bash
aigw version
aigw sync
aigw check --depth local
aigw check
```

Keep client integrations enabled across program replacement. Follow
[update and rollback](../../README.md#update-and-rollback) with the active
executable, then verify readiness. Pilot the actual release pair with retained
client state; a fresh setup or disable/re-enable cycle tests a different journey.
Configuration readability does not establish client or Provider compatibility,
and restoring the executable does not restore configuration backups.

## Staged rollout

| Stage           | Evidence                                                            |
| --------------- | ------------------------------------------------------------------- |
| Manifest review | Token-free diff and semantic validation                             |
| Pilot           | Clean install, setup, check, and rollback on each required platform |
| Team release    | Protected Forge publication and artifact verification               |
| Member adoption | Local setup/check results; no shared Token collection               |
| Closeout        | Deprecated team file/model references removed intentionally         |

## Automated rollout

Automated member setup may consume process-scoped environment Tokens. It
does not need access to a maintainer's native credential service. Repository
CI separately uses isolated fixtures under the
[quality and platform evidence policy](../governance/change-and-release-policy.md#quality-and-platform-evidence).

Set `AIGW_SECRET_BACKEND=env` and provide only the providers exercised by that
job. Environment names use `AIGW_TOKEN_<PROVIDER>` with the provider ID
uppercased and punctuation encoded by ASCII hex (`-` becomes `_2D`, `.` becomes
`_2E`, and `_` becomes `_5F`). This reversible mapping prevents two distinct
provider IDs from sharing a variable. The backend is deliberately read-only:
setup may consume present values, but rotation and deletion fail explicitly.

Scripts that read `--json` output should check each document's
`schema_version`: a document whose fields change moves to a higher version,
and the CHANGELOG lists every renamed flag and field. A `next_action` is a
complete `aigw` command that a script can run as it stands; `next_detail` is a
step for a person: one with a placeholder, in prose, or one that asks for a
value such as a Token. A removed command or flag has no hidden spelling: it is
refused as unknown, and the CHANGELOG names its replacement. A refused flag
names the command's help, such as `aigw setup --help`, as the next action. With
`--json`, wherever it stands before a `--` that ends the flags, the refusal is
a JSON failure document; `--json=false` keeps text.
