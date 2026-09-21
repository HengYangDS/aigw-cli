# Team Rollout

A team distributes reviewed public configuration; each member supplies Tokens
locally. Import does not require Tokens or installed clients; activate each
client when its prerequisites are available.

## Maintainer

1. Download the reviewed token-free
   [`manifests/team.toml`](../../manifests/team.toml).
2. Add only reviewed Account endpoints and admitted Profiles.
3. Keep Tokens, personal paths, identities, and release credentials out.
4. Validate the manifest in a clean repository environment.
5. Publish it through the team's ordinary configuration channel.

Distribute the reviewed file from a release tag or immutable commit in either
Forge, not a moving branch. Open `manifests/team.toml` at that revision and use
the Forge's raw-file download; save it as `team.toml`. Do not save the rendered
HTML page. The portable program archive does not include this team-specific
configuration. Keep the selected revision with the team's rollout instructions
so members receive the same public configuration without cloning the repository.

A manifest should contain the minimum Profile set users need. Provider catalogs
are discovery input, not automatic routing policy. Teams own model choice;
AIGW does not infer capability, quality or version policy from a model ID.
Adding a compatible model to an existing Account and admitted client changes
configuration, not the Adapter implementation. New client or authentication
boundaries follow [Adapter admission](../governance/adapter-admission.md).

### Manifest authoring contract

The team manifest is a curated catalogue, not a collection of personal notes.

- **Profile ID:** stable selection key: Account ID + `-` + the exact provider
  model ID, including its channel suffix.
- **`account`:** explicit reference to the credential-owning Account; Profiles
  remain reusable and do not declare a client.
- **`model`:** exact provider request identifier, preserving version
  punctuation and channel suffix.
- **`tier`:** optional curated role. Use `flagship` for the family's primary
  capability choice and `daily` for its balanced everyday choice. The tier is
  presentation metadata; it never changes routing or capability.
- **`protocols`:** the explicit set of wire protocols verified for that exact
  Account and model. A client may select only the intersection of its native
  protocols, the Account endpoints, and this set.
- **`label`:** human-readable identity: `Account label · Model display name`;
  append `· CHANNEL` with a separating space when needed.
- **`purpose`:** optional workflow description; omit throughout this
  model catalogue.
- **`recommendations.<client>`:** sole team recommendation owner, one Profile
  per client; recommendations do not belong in display text.

Use product capitalization, dotted display versions, uppercase channel names,
and spaces around `·`. Labels contain identity, not performance promises,
review status, or instructions. The catalogue assigns no workflow roles, so
every Profile consistently omits `purpose`; do not invent use cases to fill it.
Profile keys preserve provider spelling: `dmxapi-claude-fable-5-1` requests
`claude-fable-5-1`; its display label is `DMXAPI · Claude Fable 5.1`.
The `-cc` Profile is a separate channel, not the ordinary model.

Use the native `aigw config export` layout: version, recommendations, Accounts,
then Profiles; map keys follow stable lexical order and fields follow schema
order. Existing manifest tests check display structure and byte-identical
native export, without another formatter or model-name registry.

Verify exact provider identifiers before admitting models. A catalogue listing,
an authenticated protocol call, and a real-client journey prove different facts.
Recommendation readiness requires the latter two; a version name never proves
availability or capabilities. Missing evidence remains an open rollout task.

`aigw catalog` discovers Account catalogues; `aigw models` compares configured
Profile model IDs with the same observations. `Listed` and `Not listed` describe
catalogue membership only. Missing credentials, a failed request, an incomplete
response, or an absent catalogue endpoint remain explicit unknown observations,
not unavailable models. Neither command calls inference or proves native-client
readiness; use `aigw verify --for <client>` for the separate client proof.

### Reviewed model defaults

The catalogue contains GPT-6 Astra, GPT-5.6 Sol, Terra and Luna, Claude Fable
5.1, Opus 5 and Sonnet 5. It also offers a deliberately small two-tier set for
each general model family: Grok 4.6/4.3, Gemini 3.1 Pro Preview/3.8 Flash,
DeepSeek V4 Pro 0813/V4 Flash 0731, Qwen 3.8 Max/3.7 Plus, GLM 5.3/5.3 Flash,
and Kimi K3/K2.7 Code Highspeed. The first model in each pair is the reviewed
`flagship`; the second is `daily`. This is team curation, not a claim about
vendor pricing, benchmarks, or universal superiority.

All three configured Accounts listed those model IDs in the authenticated
catalogue observation on September 21, 2026. The selected protocols were also
tested with minimal inference calls. AIHubMix and DMXAPI used OpenAI Responses
for the 12 general profiles. UCloud used Responses except for Gemini 3.1 Pro
Preview, Gemini 3.8 Flash, and Kimi K2.7 Code Highspeed, which used Chat
Completions. Catalogue membership and one successful text call remain narrower
than complete tool, streaming, long-context, cost, or latency qualification.

DMXAPI's retained CC, SSVIP and CDX channels remain separate Profiles within
the Claude and GPT families; other Accounts use their ordinary model
identifiers. Channel names are not substitutes for the native model selected
in an existing Codex conversation.

The reviewed [DMXAPI public catalogue](https://rmb.dmxapi.cn/) lists ordinary
and CC Fable 5.1, ordinary/CC/SSVIP Opus 5 and Sonnet 5, and ordinary/CDX/SSVIP
GPT-5.6 Luna, Terra, Sol and GPT-6 Astra. All 20 entries are represented.
Luna CDX is marked supply-constrained; inclusion does not imply availability.
No Fable 5.1 SSVIP entry was listed in this observation.

The team recommends UCloud GPT-6 Astra for Codex and UCloud Claude Fable 5.1
for Claude Code. AIHubMix now includes those two models and uses the
[documented backup API domain](https://docs.aihubmix.com/en/quick-start),
`api.inferera.com`: `/v1` is the Responses base path; the Anthropic base is the
domain root. Its public model catalogue lists both identifiers. Listing and
configuration do not prove authenticated inference or real-client acceptance;
refresh that separate evidence before rollout.

The recommendation applies when its Account is connected. With another
Account, setup prefers the same model if that Account offers it, otherwise an
available Profile for that client. No provider Token is mandatory, and an
import preserves existing personal Client Bindings.

Reasoning effort remains a native client preference, outside manifest schema
version 6. The team preference is `high`: set `model_reasoning_effort = "high"`
in the active Codex Home's `config.toml`, and `"effortLevel": "high"` in the
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
client does not qualify the Fable recommendation; verify the selected Profile
with the actual client version that team members will use.

A successful short request proves only that client, Profile and invocation.
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
- preserves every reviewed Account and Profile;
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
`aigw test --for <client> --profile <profile> --token-stdin` consumes that Token
without reading or writing the credential store. Supply
`--config /absolute/path/to/config.toml` when the calling process intentionally
has no ordinary user HOME. One explicit Profile or client is required so input
cannot be reused across unrelated Accounts. This command reports HTTP endpoint
observation only; use the ordinary native-client `verify` journey for inference.

Raw stdin is the default. A broker that reads the native macOS bytes written by
the pinned `go-keyring` backend must select `--token-format go-keyring-base64`:
those bytes contain a storage envelope, not the API Token. AIGW accepts one
canonical envelope, validates the decoded Token, and rejects malformed or nested
encoding before HTTP. It does not infer a format, recursively decode, or rewrite
Keychain items. Linux and Windows native stores do not imply this macOS format.

If the catalogue is already imported, do not repeat setup. Add or replace one
Account Token, then select its Profile:

```bash
aigw rotate dmxapi
aigw use --for codex dmxapi-gpt-5.6-sol
aigw check
```

One connected Account is enough to begin. Accounts without Tokens remain
available but do not make another Account fail. With no enabled client, `check`
and `doctor` validate local configuration without requiring the recommended
Profiles' Tokens; their success is not a client or inference proof.

Interactive `aigw use --for <client> <profile>` can also prompt for that
Account's missing Token. Interactive use may prompt for the client or Profile;
non-interactive use requires both explicitly. If the Token is already available
and the binding is unchanged, selection performs no writes. A cancelled or
failed selection compensates its own credential writes; it preserves a newer
credential and reports any incomplete recovery. An output error after commit
does not undo the selection. Run `aigw status` before retrying.

Rotation validates and replaces only the selected Account's Token. It does not
select a Profile, rewrite client configuration or invoke a native client. The
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
execution or Token reads. `aigw check` adds endpoint checks; real-client
verification is separate. Configuration success alone is not authentication
or inference proof.

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
`--replace-profile <id>` after review. Tokens are neither exported nor replaced.

| Collision                          | Default behavior     | Explicit action                          |
| ---------------------------------- | -------------------- | ---------------------------------------- |
| Same semantic Account/Profile      | Reuse                | None                                     |
| Same ID, different public metadata | Stop before mutation | Review and use the specific replace flag |
| Local-only Profile not in manifest | Preserve             | Remove explicitly if obsolete            |
| Existing Token                     | Preserve             | Rotate explicitly if required            |

Import preserves existing client bindings and stores manifest recommendations
separately. Importing a recommendation does not select it. First-time setup may
bind recommendations to Profiles reachable through the Accounts explicitly
connected during that operation. `sync` never invents a binding; it reconciles
only enabled bindings. An existing selection is preserved even if its Token is
unavailable; use `aigw use --for <client> <profile>` to change it explicitly.
Client-native authentication does not require an AIGW Token. Import reconciles
enabled native projections through the ordinary guarded transaction; a failed
projection leaves the import uncommitted.

## Local choices

Do not edit the downloaded team manifest to encode a personal default, local
client path, or workstation-only endpoint. Import it as reviewed, then keep
local intent in AIGW's own configuration commands:

```bash
aigw use --for <client> <profile>
aigw account edit <account> --openai-url <url>
aigw profile add <profile> --account <account> --model <model>
```

`aigw use` changes only the named client binding. Account and Profile commands
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
| Closeout        | Deprecated manifest/profile references removed intentionally        |

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
