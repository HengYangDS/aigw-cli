## Context

On 2026-11-06 the owner reported that `aigw catalog` can only choose between `--all` and a default, and cannot name one third-party model provider. The owner called this a systemic problem and asked for a higher-level optimization of the whole CLI's product experience. The owner then ruled: complete breaking changes are allowed, converge as fast as possible to the ideal end state, ignore historical compatibility, and do not be bound by history. The same holds for every ETHOS-governed repository.

The commands grew one feature at a time, and the surface shows it. Measured on 0.6.1 with the owner's configuration (3 Accounts, 51 Routes, 4 clients):

| Symptom                                 | Evidence                                                                                                                                                                                   |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| The surface is wide                     | 24 top-level commands, 39 leaf commands, 121 flag occurrences, 41 distinct flags                                                                                                           |
| Six commands answer "does it work?"     | `status`, `doctor`, `check`, `test`, `verify`, `models`, each with its own output and exit rule                                                                                            |
| Four commands answer "which models?"    | `catalog`, `models`, `route list`, `route show`                                                                                                                                            |
| Four entries create a provider or model | `setup` (three modes), `add`, `route add`, `config import`                                                                                                                                 |
| Scope has no common form                | `rotate` and `balance` take an optional positional Account; `route add` takes `--account`; `catalog`, `models` and `route list` cannot narrow at all                                       |
| The default refuses instead of showing  | `aigw balance` with 3 Accounts fails with "account is required", and recommends the unrelated `aigw check`                                                                                 |
| A full check cannot finish              | `verify --for all` stops at Claude Desktop, whose verification always errors, so with Desktop enabled the verified snapshot is never written; the owner's configuration directory has none |
| `catalog` repeats itself                | 8 lists for 3 providers, one per protocol, with equal counts (524, 275, 662); `--all` prints 3,859 rows                                                                                    |
| `all` means two things                  | `catalog --all` selects how many rows print; `verify --for all` selects which clients run                                                                                                  |
| The user must invent names              | A Route ID (`dmxapi-gpt-6.1-sol`) restates the Account and the provider's model ID, and `add` sets the Route ID to the Account ID                                                          |
| The nouns are the implementation's      | Account, Route, projection, binding, adapter, activation, admitted, candidate reach help and output, while the owner thinks in providers, models and clients                               |
| CRUD gaps                               | No `account show`, `account add` or `account remove`; `client` has no list                                                                                                                 |
| Compatibility scaffolding               | Five configuration files exist because schema-6 readers must keep reading `config.toml`; `account rename` needs `--finalize` and two `--confirm-*` flags over about 700 lines              |
| Small habits fail                       | No dynamic shell completion at all (0 uses of `ValidArgsFunction`); no `aigw version`; no `Examples` in any help; generic "Impact" text in failures                                        |

## Goals / Non-Goals

**Goals:**

- Three nouns: provider, model and client. One name for a model: `provider/model`, which the user never invents.
- The everyday path is three commands: `aigw status`, `aigw use <client> <provider/model>`, `aigw check`.
- One grammar everywhere: objects are positional, filters are one flag vocabulary, and each noun has the same verbs.
- One ladder of evidence for "does it work?", and one local list plus one remote search for "which models?".
- One configuration file, in one format, whose shape mirrors the nouns.
- Code, configuration, JSON, specifications, documentation and output all use the same words.
- Measurable: 12 top-level commands (from 24), 32 leaf commands (from 39), 30 distinct flags (from 41), about 90 flag occurrences (from 121), and no Account, Route, projection, binding, adapter, admitted or candidate in help or human output, enforced by a test.

**Non-Goals:**

- A full-screen terminal browser. Filterable prompts and completion cover browsing; revisit on evidence.
- Changes to what a client file or its `.aigw-state.json` sidecar contains, beyond the credential reader's path. Display labels keep their `Provider · Model` form, and Hermes provider IDs keep today's spelling (`aigw-dmxapi-openai-responses`) because resumed sessions name them.
- Changes to credential storage, Keychain custody, signing, notarization or the release process.
- Runtime routing or failover between providers.

**Anti-goals, which would make this a failure even if it works:**

- A running client session that loses its credential during the upgrade.
- An alias, hidden spelling, deprecation warning or lingering reader of the old format. The one-time importer below is gone by 0.8.0.
- Prompts in `--json` or non-interactive mode.
- A preview that sends an authenticated request or leaves anything behind.

## Decisions

### Approaches

| Approach                                | What it does                                                                                                            | Trade-off                                                                                                                                                                     |
| --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A. Same objects, uniform grammar        | Renames Account to provider, unifies filters and verbs, merges overlapping commands, and leaves the configuration alone | Fastest, but the Route stays: from a catalog to real use still takes three steps and an invented name                                                                         |
| **B. Provider, model, client (chosen)** | A plus `provider/model` as the only model name, one configuration file, and adding a model as part of using it          | Largest change: configuration import, all tests and documents; it removes the most concepts and code                                                                          |
| C. B plus a full-screen browser         | An interactive browser on bare `aigw`                                                                                   | The most pleasant way to browse several hundred models, but one more interface to keep correct on three platforms, before any evidence that prompts and completion fall short |

B is the author's call under the owner's standing delegation ("make the best decisions yourself"), and the owner may overrule it. The root cause of the friction is that the CLI exposes implementation objects instead of the owner's nouns. A fixes the grammar around the same objects, and C adds a surface before that is known to be needed.

### Vocabulary

| Word     | Meaning                                                                                                                                                  | Replaces                                                   |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| provider | A service you hold a Token for: an ID, a label, one endpoint per protocol, a Token                                                                       | Account                                                    |
| model    | What a provider serves, named `provider/model`, where the part after the first `/` is the provider's own model ID, byte for byte                         | Route                                                      |
| family   | Optional: which canonical model a provider's model is, so that variants and providers can be compared (`dmxapi/gpt-6.1-sol-cdx` is family `gpt-6.1-sol`) | canonical Model                                            |
| client   | `claude`, `claude-desktop`, `codex`, `hermes`                                                                                                            | unchanged                                                  |
| protocol | `anthropic`, `responses`, `chat`                                                                                                                         | `anthropic`, `openai_responses`, `openai_chat_completions` |
| Token    | The provider's API credential                                                                                                                            | unchanged                                                  |

`provider/model` is safe as an identity. The owner's 51 Routes and the shipped manifest's 55 have no duplicate (Account, upstream ID) pair and no `/` in a model ID, and a split at the first `/` still accepts one. No client file or `.aigw-state.json` holds a Route ID, so Route IDs can go. Provider IDs keep today's identifier rule and values, because Token items, fingerprints and Hermes provider IDs are built from them.

The canonical Model stays as family because it has real consumers: Claude Desktop's one-entry-per-model list, Codex's bundled-metadata alias, and display labels. It leaves the everyday surface.

A model's `protocols` say on which endpoints AIGW may use it. Provider catalogs do not tell: on all 3 providers the model IDs are identical on every protocol, while each of the 51 Routes declares exactly one. So AIGW never infers a protocol from a catalog or a model's name. A protocol comes from the client that uses the model, from `--protocol`, or from a team file.

### Command tree

```text
Daily
  aigw                                  same as `aigw status`; without configuration, names `aigw setup`
  aigw status                           what each client uses and whether it is ready; no network
  aigw use <client> <provider/model>    point a client at a model
  aigw check [client...]                does it work? see "One ladder of evidence"

Models
  aigw model list [query]               models you have added; no network, no Token read
  aigw model search [query]             what providers offer, from their authenticated catalogs
  aigw model show <provider/model>
  aigw model add <provider/model>
  aigw model edit <provider/model>
  aigw model remove <provider/model>

Providers
  aigw provider list
  aigw provider show <provider>
  aigw provider add <provider>
  aigw provider edit <provider>
  aigw provider rotate <provider>
  aigw provider balance [provider...]   every provider by default
  aigw provider remove <provider>

Clients
  aigw client list                      every admitted client: installed path, enabled, model
  aigw client enable <client>
  aigw client disable <client>
  aigw client edit <client>             executable, config files, forwarding, retries

Maintain
  aigw setup [--from <file>]            guided first run, or a team file; safe to run again
  aigw sync                             make client files match the configuration
  aigw config path | export | rollback
  aigw self install | update | rollback | uninstall | status
  aigw completion <shell>
  aigw version
```

The hidden `aigw credential <client> <fingerprint>` stays, unchanged: client files call it.

Today's commands map onto the tree like this:

| Today                                            | End state                                                      |
| ------------------------------------------------ | -------------------------------------------------------------- |
| `doctor`                                         | `check --depth local --verbose`                                |
| `test`                                           | `check --depth endpoint`                                       |
| `verify --for <client>`                          | `check <client> --depth client`                                |
| `catalog`, `models`                              | `model search`                                                 |
| `route list`, `route show`                       | `model list`, `model show`                                     |
| `route add`, `route edit`, `route remove`        | `model add`, `model edit`, `model remove`                      |
| `route rename`, `account rename`                 | removed                                                        |
| `account list`, `account edit`                   | `provider list`, `provider edit`                               |
| `add`                                            | `provider add`, then `use`                                     |
| `rotate`, `balance`                              | `provider rotate`, `provider balance`                          |
| `account diagnostics`                            | `provider edit --balance-*`                                    |
| `client discover`                                | `client list`                                                  |
| `repair`                                         | `sync`, which already does everything `repair` does            |
| `rollback`                                       | `config rollback`                                              |
| `config import`                                  | `setup --from`                                                 |
| `config migrate`                                 | removed; `sync` imports once                                   |
| `install`, `update`, `uninstall`, `installation` | `self install`, `self update`, `self uninstall`, `self status` |
| `update --rollback`                              | `self rollback`                                                |
| `update --candidate`                             | `self update --archive`                                        |

### One flag vocabulary

| Flag                       | Meaning                                                                 | Where                                                      |
| -------------------------- | ----------------------------------------------------------------------- | ---------------------------------------------------------- |
| `[query]`                  | Case-insensitive substrings of model ID or label; every word must match | `model list`, `model search`                               |
| `--provider <id>`          | Narrow to a provider; repeatable                                        | `model list`, `model search`, `setup --from`               |
| `--client <id>`            | Narrow to models this client can use; repeatable                        | `model list`                                               |
| `--protocol <p>`           | An endpoint protocol                                                    | `model list`, `model add`, `model edit`, `use`             |
| `--family <id>`            | The model's family                                                      | `model add`, `model edit`                                  |
| `--capability <c>`         | A qualified capability; repeatable                                      | `model add`, `model edit`                                  |
| `--model <provider/model>` | Try this model without selecting it                                     | `check` only                                               |
| `--depth <d>`              | Evidence depth                                                          | `check` only                                               |
| `--endpoint <p>=<url>`     | One endpoint; repeatable                                                | `provider add`, `provider edit`                            |
| `--token-stdin`            | Read one Token from standard input                                      | `provider add`, `provider rotate`, `check`, `setup --from` |
| `--verbose`                | Print every local check                                                 | `check` only                                               |
| `--unlisted`               | Add a model that the provider's catalog does not list                   | `use`, `model add`                                         |
| `--dry-run`, `--json`      | As today                                                                | wherever the requirements below demand                     |

`--for`, `--account`, `--route`, `--upstream-model`, `--all`, `--purpose`, `--chat-url`, `--openai-url`, `--anthropic-url`, `--endpoint-only`, `--token-format`, `--config`, `--finalize`, `--confirm-*` and the `--keep-*`, `--replace-*`, `--retire-*` families go.

An unknown provider, client or model refuses with the closest matches, and a model argument without `/` lists the models it could mean.

### Which models: list and search

The split follows `brew list` and `brew search`, `mise ls` and `mise ls-remote`: list is local and instant, search goes to the network.

`aigw model list` reads only the configuration, shows each model's label, protocols and the clients using it, and marks deprecated ones.

`aigw model search` reads each selected provider's authenticated catalog and prints every match, one per line, grouped by provider. It has no compact mode and no `--all`: the narrowing is the query and `--provider`. A catalog does not say which client can use a model, so search has no `--client`.

```text
$ aigw model search sonnet --provider dmxapi --provider ucloud
DMXAPI
  dmxapi/claude-sonnet-5-5-cc      added   Claude Sonnet 5.5 · CC
  dmxapi/claude-sonnet-5-5-ssvip   added   Claude Sonnet 5.5 · SSVIP
  dmxapi/claude-sonnet-4-6         available
UCloud
  ucloud/claude-sonnet-5-5         added   Claude Sonnet 5.5
  ucloud/old-sonnet                missing

5 models from 2 providers
```

- State is `added`, `available`, `missing` (added, but the provider no longer lists it) or `deprecated`. This replaces `models` and the admitted and candidate words.
- Each provider appears once. The per-protocol reads merge into one row, and `--json` keeps which protocol source listed each model.
- Catalogs are fetched concurrently, one read per distinct protocol source, within today's 15 s bound.
- A provider without a Token, like one whose catalog cannot be read, gets one line with its fix, the other providers still print, and the command exits 1. A provider read on one protocol but not another shows the models it got, names the failed source and also exits 1.
- An added model that the provider no longer lists is `missing`, even when it is also deprecated.

### One ladder of evidence

`aigw check [client...]` covers the enabled clients by default, and named clients also when disabled. `--depth` picks how far to go, and each depth runs the ones below it:

| Depth             | Proves                                                                                                             | Today's source                  |
| ----------------- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------- |
| `local`           | Configuration valid, Token present, client files match, no conflicting `ANTHROPIC_*` or `OPENAI_API_KEY` variables | `doctor`, `status`              |
| `endpoint`        | The provider answers an authenticated model-free request                                                           | `test`, `check --endpoint-only` |
| `model` (default) | The selected model answers one bounded inference request; may use quota                                            | `check`                         |
| `client`          | A real client session completes; may use more quota                                                                | `verify`                        |

```text
$ aigw check
  ✓ Claude          ucloud/claude-opus-5-5   endpoint  Claude Code picks its own model
  ✓ Claude Desktop  ucloud/claude-opus-5-5   model
  ✓ Codex           dmxapi/gpt-6.1-sol       model
  ✗ Hermes          ucloud/claude-opus-5-5   endpoint  HTTP 401: the provider rejected the Token
    Fix: aigw provider rotate ucloud
```

- Each row names the deepest depth that client reached, so a green row never claims more than was proved. A client stops at its first failed depth, and the other clients still run; every checked client gets its row. The depth replaces today's `endpoint_checked` and `inference_checked` readiness states, and `client` marks a completed session.
- A depth that a client cannot run is skipped with its reason, and a skip is not a failure. Claude Code with its own model preference has no `model` depth, because AIGW does not choose the model it sends. Client-native and external-helper authentication have no `endpoint` or `model` depth. Claude Desktop has no `client` depth, because it needs acceptance inside the app. A named disabled client has no client files to match.
- The command exits 0 when no checked client failed, and 1 otherwise, including when no client is enabled.
- `--verbose` prints every local check, which is today's `doctor` view. `--json` always carries them.
- With a one-time Token, `local` skips its stored-Token check and gives the reason.
- `--model <provider/model>` with one named client tries that model without selecting it, replacing `--route` on `test` and `verify`. `--token-stdin` requires `--model`; it uses a one-time Token and neither reads nor writes the store (limited to depths `endpoint` and `model`). It takes the raw Token: `--token-format` goes, and a broker that reads the native store decodes its envelope itself.
- `aigw check --depth client` with no named client writes the verified snapshot that `aigw config rollback` restores, when no client failed and every client that can run a session ran one. The snapshot records each client's depth. Today this snapshot cannot exist while Claude Desktop is enabled.

`status` stays separate because it never touches the network and is what bare `aigw` shows. Its exit rule stays: 0 when the configuration loads.

### Using a model

`aigw use <client> <provider/model>` is the main act. With fewer arguments in a terminal it prompts, with a filterable list and recommended models first: no arguments asks for the client and then the model, `<client>` asks for the model, and `<provider/model>` asks for a compatible client. Non-interactive and `--json` runs need both arguments. A missing Token prompts and validates, as today.

A model that has not been added is added in the same transaction, so a search row is one command from use:

- AIGW first reads that provider's catalog. A model it does not list is refused, with the closest listed IDs. `--unlisted` skips the check for a provider whose catalog is incomplete or unreadable.
- The new model gets the client's protocol. If the client speaks several that the provider serves, `use` prompts, or takes `--protocol`.
- An added model that lacks the client's protocol gains it the same way, and the result says so. A provider with no endpoint for the client's protocols refuses.
- `family` defaults to an existing family whose ID equals the model ID ignoring case; otherwise the model is its own family. A variant such as `dmxapi/gpt-6.1-sol-cdx` is linked with `--family` or by a team file. Deriving a family from a prefix is refused on purpose: Codex aliases a family's bundled metadata, such as its context window, to the model, so a wrong guess misdescribes it silently.
- A model that `aigw use` adds declares no capabilities; `aigw model add --capability` can declare them. If that turns off Claude Desktop's tool search, which needs every listed model to declare it, the result says so and names `aigw model edit <model> --capability tool_search`.
- A preview never sends a request, so it reports the addition and says that the listing was not checked.
- A model or protocol that `use` adds also changes the files of other clients that list that provider's models, such as Hermes and Claude Desktop, and the result names them.

`aigw model add` applies the same catalog check. Its `--protocol` is optional only when the provider has one endpoint. An unknown `--family` creates that family, labelled from the model. `aigw model edit` replaces each repeatable set it is given: `--protocol` the protocols, and `--capability` the capabilities on each of the model's protocols that admits them, refusing a capability that none admits; `--capability ""` clears them. It refuses to remove a protocol that a client's selection uses: `aigw use <client> <model> --protocol <p>` moves the client first. Only a team file sets `deprecated`.

`use` keeps `--protocol` for clients that speak several protocols. Everything else about a connection moves to `aigw client edit`: `--executable`, `--target`, `--via <url>` and `--direct` (today's `--forwarding-endpoint`), `--request-retries`, `--stream-retries` and `--native-retries`. `client enable` keeps `--executable` and `--target`.

Forwarding is cleared, and the result says so, whenever the client's upstream endpoint changes: a change of provider, protocol or the provider's endpoint. Retry budgets belong to one provider: changing the model or protocol within a provider keeps them, and moving the client to another provider clears them and says so. This replaces today's refusal.

### Providers

- `aigw provider add` stores the Token, from a prompt or `--token-stdin`, and validates it with one authenticated model-free request. When the provider rejects the Token or the request cannot complete, it refuses and stores nothing. Today `add` validates nothing.
- `aigw provider balance` with no argument reports every provider. A provider without a balance probe says so without failing. A failed probe does not hide the others, and the command exits 1.
- When `aigw provider edit --endpoint` changes a protocol's endpoint, the capabilities of the provider's models on that protocol are cleared and reported, because they were qualified on the old endpoint.
- `aigw provider edit` owns precise balance credentials through `--balance-token-stdin` and `--balance-user-id`, which go together, and `--no-balance-token` removes them. In a terminal, `--balance-token-stdin` prompts with echo off, as Token input does. The three-level `account diagnostics` group goes.
- `aigw provider remove` deletes the provider, its models, its Token and its balance credentials, and refuses while a client uses one of its models. With a read-only credential backend it removes the configuration and names the variable to unset. Recommendations that name it are dropped and reported, as `route remove` does today, and `--dry-run` shows everything first. This fills a gap: no command removes an Account today.
- `provider rename` and `route rename` do not exist. A model's name is derived. Renaming a provider is rare, and `provider add`, `aigw use` and `provider remove` do it. This removes `internal/app/renaming` and both rename commands: about 700 lines of code and about 590 of tests. Author's call: the owner may keep a one-step rename instead.

### Setup

`aigw setup` becomes safe to run again and does one job: get to a working state.

- Interactive: add a provider or choose an existing one, choose an installed client, choose a model from that provider's live catalog (free text if it has none), then run `check`. With no installed client it adds the provider and stops, naming `aigw client list`. It is the one write without a preview; `setup --from --dry-run` previews a team file. Without a terminal it refuses and names `--from`; it refuses while an import is pending.
- `--from <file>`: read a team file, apply it, and connect Tokens: prompt in a terminal, `--provider <id> --token-stdin` in a script, and otherwise defer with the fix named. It replaces both `setup --from` and `config import`. For models, families and recommendations, what the file names wins, and local-only entries stay unless `--prune` removes the unused ones. A team file never changes an existing provider's endpoints or Token and never replaces a client's selection. A differing endpoint stays local and is reported, because a file must not redirect a Token: the owner's `dmxapi` points at the local proxy, and the team file at the provider. `aigw provider edit` changes an endpoint deliberately. `--dry-run` shows all of it. A Token it stores is validated as `provider add` validates one. A team file may set a balance probe on a provider that has none. The file's provider label wins, while a differing balance probe stays local and is reported, like an endpoint. A file change that would take a protocol from a client's selection keeps the local model and is reported, and so are the file's capabilities for a provider whose endpoint stays local, because they were qualified on the other endpoint. A Token that fails validation refuses the whole apply. A file with a `[clients]` section, or in another version, is refused; a version 7 file is converted by importing it with 0.6.1, upgrading, running `aigw sync` and then `aigw config export`.
- Flag-driven setup and `aigw add` go. A script writes `aigw provider add <id> --endpoint … --token-stdin` and then `aigw use`.

### One configuration file

Everything lives in one file, `aigw.toml`, in today's configuration directory, with `version = 8`. A team file is the same format without `[clients]`. The manifest and configuration version numbers merge.

```toml
version = 8

[[providers]]
id = "dmxapi"
label = "DMXAPI"
balance = { kind = "dmxapi", base_url = "https://www.dmxapi.cn" }
endpoints = { anthropic = "https://www.dmxapi.cn", responses = "https://www.dmxapi.cn/v1" }

[[providers.models]]
id = "gpt-6.1-sol-cdx"
family = "gpt-6.1-sol"
label = "DMXAPI · GPT-6.1 Sol · CDX"
protocols = { responses = [] }

[[families]]
id = "gpt-6.1-sol"
label = "GPT-6.1 Sol"

[recommendations.codex]
primary = { model = "dmxapi/gpt-6.1-sol-cdx", protocol = "responses" }
alternatives = [{ model = "ucloud/gpt-6.1-sol" }]

[clients.codex]
enabled = true
model = "dmxapi/gpt-6.1-sol"
protocol = "responses"
retries = { request = 3, stream = 3 }
forwarding = { endpoint = "http://127.0.0.1:8792/dmxapi/v1", upstream = "…", protocol = "responses" }
```

- Array order is display order, so every `order` field and its comparison code go.
- `config.forwarding.toml`, `config.retries.toml`, `config.catalog.toml` and `config.capabilities.toml` go. They exist only so that schema-6 readers can keep reading `config.toml`. This removes `store/components.go` and its neighbours.
- `purpose` goes: nothing displays it on any model today.
- `protocols` maps each protocol to its capabilities (today only `tool_search`, manifest-set). `deprecated = true` replaces `lifecycle`.

### Upgrade without breaking a running client

A running Claude Code or Codex session keeps calling the credential reader it started with. That reader is a retained copy of the old program: it reads `config.toml` and accepts only schema 6, decoded strictly. So the old file must stay readable after the upgrade, and a new file name makes that free:

1. The release ships one importer from schema 6 to `aigw.toml`. `aigw sync` runs it when `aigw.toml` is absent and `config.toml` is present, and `aigw.toml` wins if both exist. `sync --dry-run` previews it. While an import is pending, `sync`, `self …`, `version`, `completion` and `config path` still run, and completion suggests nothing. Every other command that needs configuration refuses and names `aigw sync`. Older schemas refuse and name the release that migrates them.
2. In one transaction, `sync` writes `aigw.toml`, installs the new credential reader and rewrites client files to call it.
3. `config.toml` and its component files stay untouched. Old readers in running sessions and the old program keep working from them. Token items do not change, so both programs read the same Tokens.
4. `aigw self rollback` restores the old program, which reads its old files, and its `aigw sync` points clients back at its reader. The check that the previous program can read the configuration stages the file that program reads: `config.toml` for a program older than 0.7.0, `aigw.toml` otherwise. Changes made after the import stay in `aigw.toml`, invisible to the earlier release, and `self rollback` says so; `aigw.toml` wins again on a later forward upgrade. A Homebrew rollback keeps its Keychain custody step and loses its retry-policy refusal, because the old files are intact. With an import still pending, `self rollback` reads `config.toml` in memory to name the Token items and writes nothing.

5. 0.8.0 deletes the importer and the old files. Owner: this repository; a task in the Change names it. A client session started before the 0.7.0 `sync` and still running after the 0.8.0 `sync` gets a refused credential and must restart, and `credential-continuity`'s retained-command scenario changes to say so.

A model ID is unique within its provider. The import merges schema 6 Routes that share a provider and model ID into one model whose protocols are their union; the first Route in configuration order supplies the label, each merge is reported, and Routes whose families differ refuse the import, naming both.

`aigw self rollback --dry-run` previews the retained program, its version and the file it would stage, and writes nothing. It reads the version from the program's embedded build information without running it, because a preview runs no other program, and says so when that information cannot be read.

Credential fingerprints are built from client, provider ID and upstream endpoint, so they do not change. Retries lose their connection key because they move into `[clients]`, and the importer rewrites the protocol spellings in forwarding records. Hermes provider IDs keep their spelling through a map in the Hermes adapter.

`self rollback` replaces `update --rollback` and `config migrate --rollback`. `config rollback` is today's `rollback`, with `--last-change`.

### The reader contract

Today's reader strictly decodes the whole configuration, so every schema change must keep it working, which is why four component files exist. The new file name solves that once. To solve it for good, the 0.7.0 reader decodes only what it needs and ignores everything else, including `version`: each provider's `id` and `endpoints`, the protocol keys of its models, and each client's `enabled` and `authentication`. An unknown protocol key is ignored, and an unknown authentication kind makes the reader refuse that client. The reader checks no selection, as today: a running session on a provider that its client has left keeps getting that provider's Token while the provider and endpoint exist. The fingerprint check stays exact, and a test pins this view. Everything else in `aigw.toml` can change in any release without touching a reader. Changing a frozen field takes a new file name, as this release does.

### Output and errors

- One row per item wherever it fits. Everything keeps today's renderer, glyphs, value column and `Next` section.
- Every failure says what happened and gives one exact command to run: `Problem` and `Fix`. A fix never names a command that would fail the same way: a configuration that does not load names its stage and `aigw config path`. The credential reader is the exception that proves the rule: its failures keep naming `aigw sync`, because the installed program can read a configuration that an older reader cannot, and its `sync` installs a reader that can. When the user must choose a value, the fix is a command with placeholders, carried in JSON as `next_detail`; `next_action` holds only an exact command. The generic "Impact" text goes, as do unrelated fixes such as `aigw check` for a missing argument. Specific impact stays where it adds a fact. "Quiet handled failures" and "Local failures name the failed stage" stay as they are.
- A test scans help, golden output and JSON keys for the retired words. Keychain item labels and environment-variable names are exceptions, because credential storage does not change.
- Every leaf command's help has one to three `Examples`, and help groups follow the tree above.

### Completion

Every positional and filter flag completes from local configuration without network or Token access: clients, provider IDs and `provider/model`, with `use <client> <TAB>` offering only models that client can use. Protocols and depths complete from fixed lists.

### JSON

The document contract stays: one document, `schema_version`, `ok`, `next_action`, `--json` never prompts, and the exit rule. Fields use the new words: `provider`, `model` as `provider/model`, `client`, `protocol`, `depth`. Every changed document's `schema_version` increases by one. `model search --json` keeps per-protocol observations, and `check --json` carries the depth reached and every check.

### Names in code

Types and packages use the same words: `domain.Account` becomes `domain.Provider`, `domain.Route` becomes `domain.Model`, today's `domain.Model` becomes `domain.Family`, and `internal/cli` packages follow the tree. The specification capability `route-client-selection` becomes `model-selection`. A new decision record supersedes DR-0003 (account rename) and DR-0004 (Account, Route, client binding authority); older records stay as written.

### Delivery

One Change, one lane and one release, 0.7.0. Each commit keeps the build and tests green, and the order is that of `tasks.md`:

1. The mechanical rename of Go identifiers, with no change to behavior, files or output.
2. The shared grammar and output: `provider/model` references, protocol names, Problem and Fix, and completion.
3. The commands, family by family, then the removal of the superseded ones.
4. The configuration file, its import, the reader view and the rollback staging.
5. The documents, the team file and the release notes.

The command surface lands before the configuration file, so the acceptance scripts are written once against the final commands, and the storage change then alters only files on disk. Until it lands, a `provider/model` reference resolves to the configured Route with that provider and upstream ID, and `model add` stores a Route whose derived ID never appears in output.

There is no staged compatibility and no intermediate release. Lane work continues under the normal gates: fresh-context review of each step, `mise run check`, the proposal and the hosted three-platform graph, then one proof of the lane head, delivery on both peers and the release. The removal in 0.8.0 is its own Change.

**Acceptance for 0.7.0:**

- The hosted three-platform graph and real-client journeys from v0.6.1, including the upgrade through `aigw sync` and a rollback.
- On the operator Mac through Homebrew: `aigw check --depth client` with real Claude, Codex and Hermes. A Codex session started before the upgrade still gets its credential after `aigw sync`. A Homebrew rollback to 0.6.1 and a forward upgrade keep every Token and client working.
- The installed `aigw --help` tree equals the tree above.

### Implementation map

**Packages.** Each top-level command has its own `internal/cli/<command>` package and `cmd/aigw/testdata/script/<command>.txtar`, as the architecture gate requires: `status`, `use`, `check`, `model`, `provider`, `client`, `setup`, `sync`, `config`, `self`, `completion` and `version`, plus the hidden `credential`. `internal/cli/invocation` owns argument parsing, resolution, closest-match refusals and completion.

**Names.**

- `domain.Provider`, `domain.Model` (one provider's model: provider, ID, family, label, protocols and `Deprecated`) and `domain.Family`.
- `domain.ModelRef`, from `domain.ParseModelRef(string) (domain.ModelRef, error)`, with `Provider()`, `ID()` and `String()`. It splits at the first `/`. `ClientBinding.Model` holds one, and `Runtime.Model` replaces `Runtime.RouteID`.
- `check.Depth` takes `local`, `endpoint`, `model` or `client`. Each client's result names the depth it reached, each skipped depth with its reason, and its failure if one occurred.
- The configuration directory holds `aigw.toml`, `aigw.toml.bak`, `aigw.verified.json` and `aigw.toml.lock`. The import reads `config.toml` and `config.{forwarding,retries,catalog,capabilities}.toml` and never writes them.
- The credential reader calls `store.LoadCredentialView() (domain.CredentialView, error)`.

**JSON versions.** A document that replaces others continues from the highest predecessor `schema_version` plus one, so `check` becomes 3 and `model search` 2, and a document without a predecessor starts at 1.

**Foundations land with their first consumer.** The architecture gate's `used_exports_only` rule refuses an exported identifier that nothing outside its package uses, in `internal/domain`, `internal/cli/invocation`, `internal/terminal` and every command package. So tasks 2.1, 2.2 and 2.4 land together with task 3.1, the model commands that first use them; task 2.3 lands alone, because every command already renders failures.

**Release source.** Every official archive up to 0.6.1 embeds the private GitLab peer's origin and project path as an update source, and the owner ruled on 2026-11-06 that this identity must not be public. From 0.7.0 an official build embeds only the public GitHub source, and release construction refuses an environment that sets `AIGW_GITLAB_RELEASE_ORIGIN` or `AIGW_GITLAB_RELEASE_REPOSITORY`. At run time those variables still add a GitLab peer to `aigw self update`, under the existing source validation (task 5.7).

**Global constraints.**

- Linux, macOS and Windows: the build, vet and lint pass on all three, and hosted coverage stays above 95% on each. No PowerShell or Linux-only shell scripts.
- Every Go file stays within 500 code lines, every merged specification within 500 non-blank lines, and every requirement within 500 characters.
- DR-0009 holds: no forwarding facade, alias or hidden spelling. Test readers stay package-local, with no shared test-support API.
- The command line that client files embed, `aigw credential <client> <fingerprint>`, does not change.

**Review focus.** The inputs most likely to hurt a person using the CLI, each pinned by a test in the task named:

1. A model ID that contains `/`, such as `aihubmix/openai/gpt-oss`, splits at the first `/` (2.1).
2. Input that is not a terminal, from a pipe, CI or `--json`, never prompts in `use`, `setup` or `provider add`, and each refusal names the missing argument or flag (3.3, 4.1, 4.5).
3. A provider whose catalog is unreadable or empty during `use` or `model add` gets a refusal that names `--unlisted`, and a preview reads nothing (3.1, 4.1).
4. A repeated `aigw sync`, or both files present after a rollback and a forward upgrade: `aigw.toml` wins, and nothing is imported twice (6.3).
5. Completion with a locked or failing credential store returns at once and prints nothing extra (2.4).

## Risks / Trade-offs

- **Size.** This touches every command, about 3,200 Account and 3,000 Route mentions, 14,000 lines of acceptance tests and 15 specification files. Mitigation: the mechanical rename is first and separate, and codemods carry the bulk.
- **Removing `provider rename`** costs re-entering a Token on a rare operation. It is cheap to add back if the owner wants it, and it would then be one step.
- **Auto-adding on `use`** could add a typo. The catalog check refuses unlisted IDs unless `--unlisted` is given.
- **`--unlisted` or a wrong protocol** can select a model that does not work. `use` names `aigw check <client>` as its next action; `aigw use <client> <model> --protocol <p>` moves the client, and `model edit --protocol` then drops the wrong protocol.
- **Old files stay on disk until 0.8.0.** They hold no secrets. `provider remove` deletes a Token that `config.toml` still names, so a surviving old session loses that provider, which is what removing it means.
- **A team member on 0.6.1 cannot read a new team file.** 0.6.1's refusal is frozen: it reports an unsupported manifest version and says to export with the matching release. The rollout guide tells everyone to upgrade first.
- **The verified snapshot says less for Claude Desktop.** It records that Desktop was not exercised. Today's rule writes no snapshot at all on such a machine.
- **The proxy and other governed repositories** are out of scope here, but the same ruling applies to them: their compatibility removal is separate work in their own Changes.

## Author's calls for the owner to check

- Approach B over A and C.
- No rename commands: replacing a provider is three commands, and about 1,300 lines go.
- `use` adds a model, or a protocol to a model, on the spot.
- Moving a client to another provider clears its forwarding and retries, where today it refuses.
- A depth that a client cannot run is a skip, not a failure, and the verified snapshot follows the same rule.
- A team file never changes an existing provider's endpoints or Token.
- The old files go in 0.8.0, and a session that survives both upgrades must restart.
- The credential reader decodes a frozen view and ignores `version`.
- The protocol names shorten to `responses` and `chat`, and the unused `purpose` field goes.

## Left to the builder

- Code layout inside `internal/cli`, and how the store keeps array order in memory.
- Exact column layout and wording within the rules above.
- The closest-match algorithm for "did you mean".
- How many concurrent catalog reads, within the existing bound.
