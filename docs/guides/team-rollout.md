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

### Manifest authoring contract

The team manifest is a curated catalog, not a collection of personal notes.

- **Route ID:** stable lower-case selection key, independent of the provider's
  exact-case wire spelling. An Account prefix and channel suffix may make the
  key readable without making the wire ID its authority.
- **`account`:** explicit reference to the credential-owning Account; Routes
  remain reusable and do not declare a client.
- **`model`:** canonical logical Model identity. Channel suffixes do not create
  another Model.
- **`upstream_model`:** exact identifier sent to the provider, preserving
  version punctuation and any channel suffix.
- **`interfaces`:** explicit wire protocols admitted for that exact Account
  and upstream model. A client may select only the intersection of its native
  protocols, the Account endpoints, and this set.
- **`label`:** optional display override. Ordinary Routes derive
  `Account label · Model display name` from the declared labels; channel Routes
  retain an explicit `· CHANNEL` distinction when needed.
- **`purpose`:** optional workflow description; omit throughout this
  model catalog.
- **`recommendations.<client>`:** sole team recommendation owner, with a
  primary Route and optional reviewed alternatives; recommendations do not
  belong in display text.

Use product capitalization, dotted display versions, uppercase channel names,
and spaces around `·`. Labels contain identity, not performance promises,
review status, or instructions. The catalog assigns no workflow roles, so
every Route consistently omits `purpose`; do not invent use cases to fill it.
The lower-case key `dmxapi-claude-opus-5-5` selects the exact
`upstream_model = 'claude-opus-5-5'`; its derived display label is
`DMXAPI · Claude Opus 5.5`. Another provider may require a differently cased
wire ID without changing the canonical Model or stable Route ID.
The `-cc` Route is a separate channel whose `model` remains the base logical
Model and whose `upstream_model` carries `-cc`.

Use the native `aigw config export` layout: version, recommendations, Accounts,
Models, then Routes. Export writes the declared recommendations as they are; a
member's own `aigw use` selections stay local and never become recommendations.
Map keys follow stable lexical order and fields follow schema order. Export
omits a stored Route label when it equals the derived Account and Model label.
Manifest tests check the display behavior and byte-identical native export
without another formatter or model-name registry.

Optional nonnegative Account and Model `order` values govern presentation, not
routing. The team view groups DMXAPI, UCloud, then AIHubMix; within each Account,
Anthropic Messages precedes OpenAI Responses and Chat Completions. Claude series
order is Fable, Opus, Sonnet, Haiku; GPT order is Astra, Sol, Terra, Luna. Only
qualified Claude 5.5+ and GPT 6.1+ identities appear. Missing successors are not
invented. Local presentation lives in `config.catalog.toml`, separate from the
canonical routing file so published credential readers remain compatible.
For the same reason, Route capabilities those readers do not know, such as
`tool_search`, live in `config.capabilities.toml` and apply only while the
Route keeps its Account, endpoint, upstream model and interface; `aigw sync`
moves an earlier declaration there from `config.toml`.
Hermes loads the declared names and model order, but its native picker still
ranks the current provider first and other providers by model count. AIGW does
not modify that client behavior.

Verify exact provider identifiers before adding models. `aigw model search`
reads every configured catalog of each selected provider, one read per
protocol, and prints one row per model with its state: `added`, `available`,
`missing` (added, but no longer listed) or `deprecated`. A query and
`--provider` narrow the list, and `aigw model search --json` keeps each
protocol's endpoint and which protocols listed each model. Neither view
promises every model a provider can serve privately.
Anthropic Messages, OpenAI Chat Completions, and OpenAI Responses observations
are not interchangeable. A Chat Completions result cannot qualify Responses;
a Responses text call cannot qualify reasoning, tools, continuation, or
compaction. Judge a dated or suffixed identifier by the vendor's release notes,
not its name: a dated refresh such as `deepseek-v4.1-flash-1017` can be the
stronger model, while a serving tier such as `-cc` or `-free` is not.

Search compares each added model's exact provider model ID with the catalogs.
`available` and `missing` describe catalog membership only: a missing model is
reported for requalification without changing configuration. A provider
without a Token, or whose catalog cannot be read, gets one line with its fix
while the other providers still print, and the command exits 1; that is an
unknown observation, not an unavailable model.

Adding a model to the team file requires a successful authenticated inference
call carrying the exact provider, protocol, and model ID, plus compatibility
with the selected client and its native verification where required. A
catalog listing alone never qualifies a model. Deprecation sets
`lifecycle = "deprecated"` in the team file, removes the model from
recommendations, and preserves existing selections. Retirement removes the
model explicitly, with `aigw model remove` locally, after no selection or
recommendation depends on it. A search performs none of those transitions.

### Reviewed model defaults

The tracked [team file](../../manifests/team.toml) is the current model
inventory. It currently contains qualified GPT-6.1 Sol and Terra and Claude
Opus, Sonnet, and Haiku 5.5, plus one reviewed general family per other
qualified vendor. Fable, Astra, and Luna await qualified identities meeting the version
floors.
The [dated qualification
evidence](../research/provider-model-qualification.md) explains the observed
IDs, protocol tests, exclusions, and limits. A catalog listing is not a live
model or native-client availability guarantee.

Claude Code and Claude Desktop recommend DMXAPI Opus 5.5 before UCloud Opus 5.5.
Codex recommends `dmxapi/gpt-6.1-sol-cdx` before `ucloud/gpt-6.1-sol`; Hermes
recommends DMXAPI's base `dmxapi/gpt-6.1-sol` before the UCloud base model.
AIHubMix models remain available but are manual-only, not automatic
alternatives. The Codex and Hermes DMXAPI models belong to the same family,
but are distinct models. A shared family does not make every provider and
client pair interchangeable. A sole DMXAPI or UCloud provider is enough for the declared
recommendation; select an AIHubMix model explicitly with `aigw use`.
Display order never creates automatic recovery when the selected provider fails.

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
no exact projection can be produced for the selected model. It never invents
an entry for an unknown model or treats a provider `/models` response as
Codex-native metadata.
An explicit user-owned `model_catalog_json` may mask newer models bundled with
Codex. AIGW preserves that setting; if Codex reports fallback model metadata,
review the catalog before deliberately updating or removing it.

Recommendations apply only to unselected clients at setup or sync; they do not
switch a running request or replace a client's explicit selection. AIHubMix uses the
[documented API endpoint](https://docs.aihubmix.com/en/quick-start),
`api.inferera.com`: `/v1` is the Responses and Chat Completions base path; the
Anthropic base is the domain root. A successful minimal request remains narrower than full real-client
tool, continuation, streaming, and long-context acceptance.

Setup selects only a usable declared primary model or ordered alternative.
Other models remain available for explicit use, not automatic fallback by
family or identifier. No provider Token is mandatory, and a team file
preserves each client's existing selection.

Reasoning effort remains a native client preference, outside manifest schema
version 7. Choose a level supported by the selected model through Codex's
`model_reasoning_effort` or Claude Code's `effortLevel` in their active settings.
Merge those fields; do not replace either document. Applying the team file
does not set these preferences.

Context capacity and compaction are also client settings. Retain the installed
client's model metadata unless the selected provider's larger limit has been
verified. A catalog listing or a successful short request does not prove a
full-window request will succeed.

- **Codex:** `model_context_window` declares the available capacity;
  `model_auto_compact_token_limit` sets the compaction threshold. The optional
  `model_auto_compact_token_limit_scope` selects `total` (the default, counting
  the full active context) or `body_after_prefix` (growth after the carried
  compaction-window prefix). Changing the counting scope does not enlarge the
  model's window. See the [Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).
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
- connects no provider unless a Token already exists or the user names one;
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
control characters fail before validation or storage.

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
aigw use codex dmxapi/gpt-6.1-sol-cdx
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
Responses endpoint, select and check its GPT-6.1 Sol model without changing
Codex or Claude selections:

```bash
aigw use hermes dmxapi/gpt-6.1-sol
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
Team-file setup is declarative: it applies the file and writes the files of
installed clients with a locally available Token without contacting the
provider. This does not prove that the Token is accepted or that inference
works. Run `aigw check` afterwards; it reports unreachable endpoints and
rejected Tokens without exposing credential values.

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
Token receives a credential helper; client-native authentication continues to
use the client's own:

```bash
aigw sync
aigw check
aigw check <client> --depth client
```

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
local-only entries stay unless `--prune` removes the models that no client
selects and no recommendation names, and then the families that no model uses.
A team file never changes an existing provider's endpoints or Token and never
replaces a client's selection:

| Difference                              | Behavior                                                           |
| --------------------------------------- | ------------------------------------------------------------------ |
| A provider that only the file has       | Added, without a Token                                             |
| An existing provider's label            | The file's label wins                                              |
| An existing provider's endpoint         | The local endpoint stays and is reported                           |
| A balance probe that the provider lacks | Added from the file; a differing probe stays local and is reported |
| A model, family or recommendation       | The file's version wins                                            |
| A local-only model or family            | Kept, unless `--prune` removes the unused ones                     |
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
explicitly. Client-native authentication does not require an AIGW Token.

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
`aigw model edit` changes a model's protocols, family, capabilities or label.

`aigw use` changes only the named client. Provider and model commands change
local configuration and are not written back into the
[distributed team file](../../manifests/team.toml). To publish a team change,
review the team file itself and distribute the new token-free revision.

## Release installation

GitLab and GitHub are independent release sources. Verify one complete artifact
set from one source; do not mix a tag, checksum file, and archive across Forges.

| Platform | Install asset                                  |
| -------- | ---------------------------------------------- |
| macOS    | Matching Darwin archive and checksum manifest  |
| Linux    | Matching Linux archive and checksum manifest   |
| Windows  | Matching Windows archive and checksum manifest |

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
| Closeout        | Deprecated manifest/route references removed intentionally          |

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
names the command's help, such as `aigw setup --help`, as the next action; the
refusal is a JSON failure document only when `--json` comes before the refused
flag.
