# Team Rollout

A team distributes reviewed public configuration; each member supplies Tokens
locally. Import does not require Tokens or installed clients; activate each
client when its prerequisites are available.

## Maintainer

1. Download the reviewed token-free
   [`manifests/team.toml`](../../manifests/team.toml).
2. Add only reviewed Account endpoints and admitted Routes.
3. Keep Tokens, personal paths, identities, and release credentials out.
4. Validate the manifest in a clean repository environment.
5. Publish it through the team's ordinary configuration channel.

Distribute the reviewed file from a release tag or immutable commit in either
Forge, not a moving branch. Open `manifests/team.toml` at that revision and use
the Forge's raw-file download; save it as `team.toml`. Do not save the rendered
HTML page. The portable program archive does not include this team-specific
configuration. Keep the selected revision with the team's rollout instructions
so members receive the same public configuration without cloning the repository.

A manifest should contain the minimum Route set users need. Provider catalogs
are discovery input, not automatic routing policy. Teams own model choice;
AIGW does not infer capability, quality or version policy from a model ID.
Adding a compatible model to an existing Account and admitted client changes
configuration, not the Adapter implementation. New client or authentication
boundaries follow [Adapter admission](../governance/adapter-admission.md).

### Manifest authoring contract

The team manifest is a curated catalogue, not a collection of personal notes.

- **Route ID:** stable selection key: Account ID + `-` + the exact provider
  wire model ID, including its channel suffix.
- **`account`:** explicit reference to the credential-owning Account; Routes
  remain reusable and do not declare a client.
- **`model`:** canonical logical Model identity. Channel suffixes do not create
  another Model.
- **`upstream_model`:** exact identifier sent to the provider, preserving
  version punctuation and any channel suffix.
- **`interfaces`:** explicit wire protocols admitted for that exact Account
  and upstream model. A client may select only the intersection of its native
  protocols, the Account endpoints, and this set.
- **`label`:** human-readable identity: `Account label · Model display name`;
  append `· CHANNEL` with a separating space when needed.
- **`purpose`:** optional workflow description; omit throughout this
  model catalogue.
- **`recommendations.<client>`:** sole team recommendation owner, one Route
  per client; recommendations do not belong in display text.

Use product capitalization, dotted display versions, uppercase channel names,
and spaces around `·`. Labels contain identity, not performance promises,
review status, or instructions. The catalogue assigns no workflow roles, so
every Route consistently omits `purpose`; do not invent use cases to fill it.
Route keys preserve provider spelling: `dmxapi-claude-fable-5-1` requests
`claude-fable-5-1`; its display label is `DMXAPI · Claude Fable 5.1`.
The `-cc` Route is a separate channel whose `model` remains the base logical
Model and whose `upstream_model` carries `-cc`.

Use the native `aigw config export` layout: version, recommendations, Accounts,
then Routes; map keys follow stable lexical order and fields follow schema
order. Existing manifest tests check display structure and byte-identical
native export, without another formatter or model-name registry.

Verify exact provider identifiers before admitting Routes. `aigw catalog`
observes every configured catalogue surface separately and records its Account,
protocol, endpoint, normalized IDs, and deterministic observation identity.
Anthropic Messages, OpenAI Chat Completions, and OpenAI Responses observations
are not interchangeable. A Chat Completions result cannot qualify Responses;
a Responses text call cannot qualify reasoning, tools, continuation, or
compaction.

`aigw models` compares each configured Route's exact `upstream_model` against
the observation for that Route's protocol. `Listed` and `Not listed` describe
catalogue membership only. New IDs are candidates; missing admitted Routes are
reported for requalification without changing configuration. Missing
credentials, a failed request, an incomplete response, or an absent catalogue
endpoint remain unknown observations, not unavailable models.

Qualification requires `aigw verify --for <client>` and the capability-specific
native evidence required by the selected client. Admission is a reviewed Model
and Route change. Deprecation sets `lifecycle = "deprecated"`, removes the Route
from recommendations, and preserves existing explicit bindings. Retirement is
an explicit Route removal after no binding or recommendation depends on it.
Catalogue refresh performs none of those transitions.

### Reviewed model defaults

The catalogue contains only GPT-6 Astra, Sol, and Luna in the GPT family and
Claude Fable 5.1, Opus 5.5, and Sonnet 5 in the Claude family. Opus 5.5 is a
known Model identity without an admitted Route until an exact Account,
protocol, and wire ID pass authenticated inference. Each other admitted
general-model family retains one previously qualified logical Model across
configured Accounts, with separately evidenced Routes: Grok 4.6, Gemini 3.1
Pro Preview, DeepSeek V4 Pro 0813, Qwen 3.8 Max, GLM 5.3, and Kimi K3. Model
entries carry identity only; client-scoped
recommendations express preference without a global benchmark or cost claim.

All three configured Accounts listed the retained September 21 set in
authenticated catalogue observations, and their selected protocols completed
minimal inference calls at that time. On September 23, 2026, UCloud also
listed `gpt-6-sol` and `gpt-6-luna`; both completed minimal OpenAI Responses
requests and are now Codex and Hermes choices, with Sol as primary. The two
Opus 5.5 wire IDs tried on UCloud on that date were rejected. That bounded
observation does not establish permanent absence. Catalogue membership and
one successful text call remain narrower than complete tool, streaming,
long-context, cost, or latency qualification.

DMXAPI's retained CC, SSVIP, and CDX channels remain separate Routes within
the Claude and GPT families. Each Route retains its exact wire ID while its
`model` names the base logical Model; a channel is not another logical model.
Other Accounts use their ordinary model identifiers. Channel names are not
substitutes for the native model selected in an existing Codex conversation.

The reviewed [DMXAPI public catalogue](https://rmb.dmxapi.cn/) listed ordinary
and CC Fable 5.1, ordinary/CC/SSVIP Sonnet 5, and ordinary/CDX/SSVIP GPT-6
Astra. The current manifest keeps those previously qualified channels. Opus 5
channels are outside the requested logical set; no Opus 5.5 channel has been
admitted. An unauthenticated DMXAPI model request returning 401 establishes
neither presence nor absence of an individual model.

The public AIHubMix `/v1/models` response observed on September 24, 2026,
listed Opus 5.5, all three GPT-6 IDs, `minimax-m3`,
`deepseek-v4.1-flash`, and Doubao Seed 2.1 Pro. UCloud's public
`/v1/models` response listed `MiniMax-M3`, `deepseek-v4.1-flash`, Doubao
Seed 2.1 Pro, and `MiniMax-H3-Max`. These listings are discovery evidence,
not newly qualified Routes.

A second read on September 24, 2026 at 10:28 UTC returned 416
[AIHubMix catalogue IDs](https://api.inferera.com/v1/models) and 132
[UCloud catalogue IDs](https://api.modelverse.cn/v1/models). AIHubMix also
listed `cc-minimax-m3`, `coding-minimax-m3`, and `grok-4.7`. The public UCloud
response listed no GPT-6 or Claude IDs; that omission does not contradict
prior authenticated UCloud inference. Neither catalogue identifies a model's
protocol, and a prefixed MiniMax wire ID is not a separate logical Model by
itself.

MiniMax's [H3 announcement](https://www.minimax.io/blog/minimax-h3)
describes a multimodal generation model that outputs video with sound. H3
family names in a provider catalogue are not evidence of a general text
model. Its [M3 announcement](https://www.minimax.io/blog/minimax-m3)
positions M3 as an LLM for coding and agentic work. AIHubMix's `minimax-m3`
and UCloud's `MiniMax-M3` are general-text candidates pending inference.

DeepSeek's [V4.1 Flash announcement](https://www.deepseek.com/en/news/deepseek-v4-1-flash/)
claims benchmark results ahead of V4 Pro and says `deepseek-v4-pro` requests
on DeepSeek's own API now route to V4.1 Flash. That does not establish what
an aggregator serves for `deepseek-v4-pro-0813`. Probe the exact
`deepseek-v4.1-flash` ID on AIHubMix and UCloud first. Preserve one DeepSeek
logical Model globally: switch only after DMXAPI also passes, or after an
explicit reviewed decision to retire its incumbent Route and lose that
Account coverage. Until then, keep the current Pro 0813 Routes; do not publish
a mixed Pro/V4.1 two-model catalogue. Do not rank model quality from `Pro`,
`Max`, or `Flash` in an ID.

The public UCloud response is not a complete substitute for the earlier
authenticated GPT and Claude observations. Admit any new Route only after a
bounded noninteractive call to its exact Account, protocol, and wire model
succeeds.

The team recommends UCloud GPT-6 Sol for Codex and Hermes, and UCloud Claude
Fable 5.1 for Claude Code and Claude Desktop. AIHubMix includes GPT-6 Astra
and Claude Fable 5.1 and uses the
[documented backup API domain](https://docs.aihubmix.com/en/quick-start),
`api.inferera.com`: `/v1` is the Responses base path; the Anthropic base is the
domain root. Its public model catalogue lists both identifiers. Listing and
configuration do not prove authenticated inference or real-client acceptance;
refresh that separate evidence before rollout.

The recommendation applies when its Account is connected. With another
Account, setup prefers the same model if that Account offers it, otherwise an
available Route for that client. No provider Token is mandatory, and an
import preserves existing personal Client Bindings.

Reasoning effort remains a native client preference, outside manifest schema
version 7. The team preference is `medium`: set `model_reasoning_effort = "medium"`
in the active Codex Home's `config.toml`, and `"effortLevel": "medium"` in the
active Claude configuration directory's `settings.json`. Merge those fields
into existing settings; do not replace either document. Importing the team
manifest does not set these preferences.

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

Check `codex --version` or `claude --version` and run `aigw verify --for <client>`
before rollout. A provider may require a newer client even when an API request
works. Update through the existing installation owner rather than adding a
second executable. Claude Code's
[`stable` and `latest` channels](https://code.claude.com/docs/en/setup#update-claude-code)
are distinct; choose deliberately when compatibility requires a channel change.
The recommended [Fable 5.1](https://code.claude.com/docs/en/model-config#work-with-fable)
requires Claude Code **2.1.257 or later**. A passing Sonnet request on an older
client does not qualify the Fable recommendation; verify the selected Route
with the actual client version that team members will use.

A successful short request proves only that client, Route and invocation.
It does not establish Desktop behavior, other operating systems, full-window
capacity, tool replay, or installation and update correctness. Verify those
journeys separately when the rollout depends on them.

## New member

First follow [installation and command discovery](../../README.md#install).
The examples below use `aigw` after its installed directory is on `PATH`; the
installed executable's explicit path works with the same arguments. Importing
team configuration does not install the program or change shell discovery.

Import the reviewed catalogue without requiring every provider Token or either
supported client. Save the maintainer's `team.toml` in the current directory,
or pass its actual path to `--from`:

```bash
aigw setup --from team.toml
```

Setup:

- validates all public metadata first;
- preserves every reviewed Account and Route;
- connects no Account unless a Token already exists or the user selects one;
- configures only installed admitted clients;
- compensates failed projections only where its owned writes remain unchanged,
  preserving newer external edits and reporting recovery conflicts.

To connect one Account during first-time setup, use this form **instead of**
the import-only command above. The rest remain optional:

```bash
aigw setup --from team.toml --account dmxapi
aigw check
```

The interactive command prompts only for the selected Account. Automation may
pipe exactly one Token by adding `--token-stdin`; it must keep `--account` so
the Token owner is explicit. The input is read through EOF and is limited to
64 KiB, including an optional terminal LF or CRLF. A Token contains only visible
ASCII characters; embedded whitespace, extra lines and control characters fail
before validation or storage.

For a one-time endpoint test,
`aigw test --for <client> --route <route> --token-stdin` consumes that Token
without reading or writing the credential store. Supply
`--config /absolute/path/to/config.toml` when the calling process intentionally
has no ordinary user HOME. One explicit Route or client is required so input
cannot be reused across unrelated Accounts. This command reports HTTP endpoint
observation only; use the ordinary native-client `verify` journey for inference.

Raw stdin is the default. A broker that reads the native macOS bytes written by
the pinned `go-keyring` backend must select `--token-format go-keyring-base64`:
those bytes contain a storage envelope, not the API Token. AIGW accepts one
canonical envelope, validates the decoded Token, and rejects malformed or nested
encoding before HTTP. It does not infer a format, recursively decode, or rewrite
Keychain items. Linux and Windows native stores do not imply this macOS format.

If the catalogue is already imported, do not repeat setup. Add or replace one
Account Token, then select its Route:

```bash
aigw rotate dmxapi
aigw use --for codex dmxapi-gpt-6-astra
aigw check
```

One connected Account is enough to begin. Accounts without Tokens remain
available but do not make another Account fail. With no enabled client, `check`
and `doctor` validate local configuration without requiring the recommended
Routes' Tokens; their success is not a client or inference proof.

Interactive `aigw use --for <client> <route>` can also prompt for that
Account's missing Token. Interactive use may prompt for the client or Route;
non-interactive use requires both explicitly. If the Token is already available
and the binding is unchanged, selection performs no writes. A cancelled or
failed selection compensates its own credential writes; it preserves a newer
credential and reports any incomplete recovery. An output error after commit
does not undo the selection. Run `aigw status` before retrying.

Rotation validates and replaces only the selected Account's Token. It does not
select a Route, rewrite client configuration or invoke a native client. The
credential helper reads the new Token when next invoked; client caching may
require a reload. Use `aigw sync` for configuration changes. Failed storage
updates use guarded compensation, preserving newer Tokens rather than
overwriting them.

## Install a client later

Claude Code, Claude Desktop, Codex, and Hermes are not setup prerequisites.
After installing a selected client, run `aigw sync`; AIGW rediscovers admitted
clients and converges only its owned configuration. Claude Desktop uses
`Claude-3p/configLibrary`, independently of Claude Code's `~/.claude` settings.
If a client has a selected Client Binding and usable authentication, sync
enables its Adapter and writes only that Adapter's owned projection.
Account-Token Client Bindings receive a credential helper; client-native bindings continue
to use the client's own authentication:

```bash
aigw sync
aigw check
aigw verify --for codex
```

`aigw status` observes selection and projection readiness without client
execution or Token reads. `aigw check` adds one bounded inference request per
eligible selected Route by default; `--endpoint-only` keeps the model-free
authentication check. Real-client verification remains separate. Configuration
success alone is not authentication or inference proof.

Optional balance credentials do not participate in `aigw check`, in either
human or JSON output. Use `aigw account diagnostics enable <account>` to configure them
and `aigw balance <account>` to request provider diagnostics. An unavailable
balance service does not make a working Client Binding unhealthy.

Claude Code and Account-Token Codex bindings use projection-matching helpers.
Changing Account or endpoint invalidates a retained helper invocation: run
`aigw sync` and reload the client's configuration. The helper does not return a
new Account's Token to a client retaining the old endpoint.

## Existing member

Export local public metadata for comparison, then import the reviewed manifest:

```bash
aigw config export > local-manifest.toml
aigw config import manifest.toml
```

Review the exported file against the incoming manifest before importing.
`config import` applies a merge; it has no preview or JSON-output mode.
Conflicting public metadata requires an explicit `--replace-account <id>` or
`--replace-route <id>` after review. Tokens are neither exported nor replaced.

| Collision                          | Default behavior     | Explicit action                          |
| ---------------------------------- | -------------------- | ---------------------------------------- |
| Same semantic Account/Route        | Reuse                | None                                     |
| Same ID, different public metadata | Stop before mutation | Review and use the specific replace flag |
| Local-only Route not in manifest   | Preserve             | Remove explicitly if obsolete            |
| Existing Token                     | Preserve             | Rotate explicitly if required            |

Import preserves existing client bindings and stores manifest recommendations
separately. Importing a recommendation does not select it. First-time setup may
bind recommendations to Routes reachable through the Accounts explicitly
connected during that operation. `sync` never invents a binding; it reconciles
only enabled bindings. An existing selection is preserved even if its Token is
unavailable; use `aigw use --for <client> <route>` to change it explicitly.
Client-native authentication does not require an AIGW Token. Import reconciles
enabled native projections through the ordinary guarded transaction; a failed
projection leaves the import uncommitted.

## Local choices

Do not edit the downloaded team manifest to encode a personal default, local
client path, or workstation-only endpoint. Import it as reviewed, then keep
local intent in AIGW's own configuration commands:

```bash
aigw use --for <client> <route>
aigw account edit <account> --openai-url <url>
aigw route add <route> --account <account> --model <model>
```

`aigw use` changes only the named client binding. Account and Route commands
change local configuration and are not written back into [distributed team manifest](../../manifests/team.toml).
To publish a team change, review the manifest itself and distribute the new
token-free revision.

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
aigw --version
aigw sync
aigw doctor
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

Set `AIGW_SECRET_BACKEND=env` and provide only the Accounts exercised by that
job. Environment names use `AIGW_TOKEN_<ACCOUNT>` with the manifest Account ID
uppercased and punctuation encoded by ASCII hex (`-` becomes `_2D`, `.` becomes
`_2E`, and `_` becomes `_5F`). This reversible mapping prevents two distinct
Account IDs from sharing a variable. The backend is deliberately read-only:
setup may consume present values, but rotation and deletion fail explicitly.
