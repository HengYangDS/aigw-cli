<!--
---
subject: aigw:team-rollout
role: how-to
state: canonical
relations:
  canonical_for: team setup and rollout
---
-->

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
  model catalogue.
- **`recommendations.<client>`:** sole team recommendation owner, with a
  primary Route and optional reviewed alternatives; recommendations do not
  belong in display text.

Use product capitalization, dotted display versions, uppercase channel names,
and spaces around `·`. Labels contain identity, not performance promises,
review status, or instructions. The catalogue assigns no workflow roles, so
every Route consistently omits `purpose`; do not invent use cases to fill it.
The lower-case key `dmxapi-claude-fable-5-1` selects the exact
`upstream_model = 'claude-fable-5-1'`; its derived display label is
`DMXAPI · Claude Fable 5.1`. Another provider may require a differently cased
wire ID without changing the canonical Model or stable Route ID.
The `-cc` Route is a separate channel whose `model` remains the base logical
Model and whose `upstream_model` carries `-cc`.

Use the native `aigw config export` layout: version, recommendations, Accounts,
Models, then Routes. Map keys follow stable lexical order and fields follow
schema order. Export omits a stored Route label when it equals the derived
Account and Model label. Manifest tests check the display behavior and
byte-identical native export without another formatter or model-name registry.

Verify exact provider identifiers before admitting Routes. `aigw catalog`
observes every configured catalogue surface separately and records its Account,
protocol, endpoint, normalized IDs, and deterministic observation identity.
The default human view expands admitted and deprecated IDs and counts other
candidates. Use `aigw catalog --all` to see every observed candidate, or
`aigw catalog --json` for the complete machine-readable observation. Neither
view promises every model a provider can serve through private routes.
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

Route admission requires a successful authenticated inference call carrying
the exact Account, protocol, and `upstream_model`, plus compatibility with the
selected client and its native verification where required. A directory
listing alone never admits a Route. Admission is a reviewed Model and Route
change. Deprecation sets `lifecycle = "deprecated"`, removes the Route
from recommendations, and preserves existing explicit bindings. Retirement is
an explicit Route removal after no binding or recommendation depends on it.
Catalogue refresh performs none of those transitions.

### Reviewed model defaults

The tracked [team manifest](../../manifests/team.toml) is the current Route
inventory. It selects Astra, Luna, and GPT-6.1 Sol, three Claude models, and one reviewed
general Model per other admitted vendor; the [dated qualification
evidence](../research/provider-model-qualification.md) explains the observed
IDs, protocol tests, exclusions, and limits. A catalogue listing is not a live
Route or native-client availability guarantee.

Claude Code and Claude Desktop initially prefer DMXAPI Opus 5.5, then UCloud,
then AIHubMix. Codex and Hermes prefer DMXAPI GPT-6.1 Sol, then AIHubMix
GPT-6.1 Sol; a sole UCloud Account selects GPT-6 Astra. A shared model name
does not make every Account/client pair interchangeable. A sole
connected Account remains sufficient. Luna remains separately selectable,
not an automatic recovery Route when the selected provider later fails.
A local Account endpoint override is not silently replaced by the team's
direct endpoint. Re-importing the team manifest
against that differing Account fails closed; inspect it with
`aigw config export` and replace the Account only when intentionally adopting
the team's direct endpoint.

Codex can run a compatible Responses Route even when the model is absent
from its native `/model` chooser. Select it with
[`aigw use --for codex`](../../README.md#use-it-every-day), then verify through
Codex; the [dated client observation](../research/provider-model-qualification.md#codex-native-chooser)
does not establish current upstream availability.
An explicit user-owned `model_catalog_json` may mask newer models bundled with
Codex. AIGW preserves that setting; if Codex reports fallback model metadata,
review the catalog before deliberately updating or removing it.

Recommendations apply only to unselected clients at setup or sync; they do not
switch a running request or replace an explicit Client Binding. AIHubMix uses the
[documented backup API domain](https://docs.aihubmix.com/en/quick-start),
`api.inferera.com`: `/v1` is the Responses and Chat Completions base path; the
Anthropic base is the domain root. A successful minimal request remains narrower than full real-client
tool, continuation, streaming, and long-context acceptance.

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

Check `codex --version`, `claude --version`, or `hermes --version` for the
installed CLI, then run `aigw verify --for <client>` for each enabled client
before rollout. Claude Desktop needs separate application-version and native
verification evidence. A provider may require a newer client even when an API
request works. Update through the existing installation owner rather than
adding a second executable. Claude Code's
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

Import the reviewed catalogue without requiring every provider Token or any
installed client. Save the maintainer's `team.toml` in the current directory,
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

The interactive command prompts only for the selected Account with a writable
backend. In read-only `env` mode, set `AIGW_TOKEN_DMXAPI` in the invoking process;
`--token-stdin` cannot store it. With a writable backend, automation may pipe
exactly one Token by adding `--token-stdin`; it must keep `--account` so the
Token owner is explicit. The input is read through EOF and is limited to
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

If the catalogue is already imported, do not repeat setup. With the
environment backend, supplying one compatible Account variable and running
`aigw sync` selects unbound reviewed recommendations and projects available
clients. With a writable backend, store the Token and select its Route
explicitly:

```bash
aigw rotate dmxapi
aigw use --for codex dmxapi-gpt-6-astra
aigw check
```

One connected Account is enough to begin. Accounts without Tokens remain
available but do not make another Account fail. With no enabled client, `check`
returns a deferred, nonzero result without probing a provider. `doctor` may
pass local diagnostics, but reports zero enabled clients and does not claim
client or inference readiness. Neither command requires Tokens for unselected
recommended Routes.

If none is connected yet, setup, status, check, and sync preview present the
same choice of compatible Accounts. A writable credential store names each
`aigw rotate <account>` command; the environment backend names each variable.
With a writable store, rotation only writes the Token; follow its explicit
`aigw use --for ...` action to select a client Route. With the environment
backend, set a compatible variable before synchronizing. Import alone does not
make `aigw sync` useful work.

When a Route is selected but its native client projection is deferred, setup,
`use`, sync preview, status, check, and doctor retain that selection. If its
Account Token is available, install the client if needed, then run `aigw sync`.
If the Token has disappeared, restore that selected Account's Token first;
the missing Token and deferred projection remain distinct in machine-readable
status. `check` does not probe a provider until an enabled client has a usable
projection; `doctor` can pass local diagnostics without claiming client
readiness.

Interactive `aigw use --for <client> <route>` can also prompt for that
Account's missing Token. Interactive use may prompt for the client or Route;
non-interactive use requires both explicitly. If the Token is already available
and the binding is unchanged, selection performs no writes. A cancelled or
failed selection compensates its own credential writes; it preserves a newer
credential and reports any incomplete recovery. An output error after commit
does not undo the selection. Run `aigw status` before retrying.

### Hermes protocol and model selection

With Hermes installed and a connected DMXAPI Account using the team's direct
Responses endpoint, select and verify its GPT-6.1 Sol Route without changing
Codex or Claude bindings:

```bash
aigw use --for hermes dmxapi-gpt-6.1-sol
aigw verify --for hermes
```

If this machine explicitly points the DMXAPI Account at an external Proxy,
qualify that endpoint separately before selecting this Route; importing the
team manifest does not silently replace the local endpoint.

The Hermes projection includes compatible models from connected Accounts;
selecting one active model does not remove the others. A Route may expose more
than one protocol to Hermes. If no previous binding or team recommendation
resolves that choice, interactive `use` asks which protocol to use;
non-interactive use requires one explicitly:

```bash
aigw use --for hermes --protocol openai_responses <route>
```

The choice uses the same `anthropic`, `openai_responses`, and
`openai_chat_completions` names as `team.toml`; AIGW never guesses from a Model
name. Direct single-Route `setup` also accepts `--protocol` when more than one
endpoint URL is supplied. Its `--chat-url` supplies an OpenAI Chat Completions
endpoint for Hermes; the protocol is stored in that Client Binding.
Guided single-Route setup and interactive selection validate the selected
protocol and endpoint before activation. Manifest setup is declarative: it
imports the catalogue and projects installed clients with a locally available
Token without contacting the provider. This does not prove that the Token is
accepted or that inference works. Run `aigw check` after import; it reports
unreachable endpoints and rejected Tokens without exposing credential values.

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
aigw verify --for <client>
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

For unattended setup, pipe one platform system-token line to
`aigw account diagnostics enable <account> --system-token-stdin --user-id <id>`.
This optional credential is separate from the Account API Token; incomplete
flag pairs fail before standard input is read.

Claude Code, Hermes, and Account-Token Codex bindings use
projection-matching helpers.
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
The import reports public changes without guessing Token or client readiness;
run `aigw status` for the selected Route's next step. Use `aigw setup --from`
when guided team onboarding is wanted instead.

| Collision                          | Default behavior     | Explicit action                          |
| ---------------------------------- | -------------------- | ---------------------------------------- |
| Same semantic Account/Route        | Reuse                | None                                     |
| Same ID, different public metadata | Stop before mutation | Review and use the specific replace flag |
| Local-only Route not in manifest   | Preserve             | Remove explicitly if obsolete            |
| Existing Token                     | Preserve             | Rotate explicitly if required            |

Import preserves existing client bindings and stores manifest recommendations
separately. Importing a recommendation does not select it. First-time setup may
bind recommendations to Routes reachable through the Accounts explicitly
connected during that operation. Later `sync` may select an unbound reviewed
recommendation when its Account Token appears in the read-only environment
backend; it does not search unselected native credentials. An existing
selection is preserved even if its Token is unavailable, and a manual-only
Route is never selected automatically. Use `aigw use --for <client> <route>`
to change a selection explicitly.
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
