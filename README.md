# AIGW CLI

AIGW is a local-first control plane for teams that use reviewed third-party AI
services. It keeps providers, their Tokens and their models, points each client
at one model, and writes only AIGW's own settings into each client's files. It
does not relay model traffic, run a gateway, or own conversation state.

Each client resolves its endpoint and model from its own selection. Read the
[authority boundary](docs/architecture/authority-and-projection-boundary.md) for
the complete design.

## Start here

| Goal                                | Command                              | Then                                 |
| ----------------------------------- | ------------------------------------ | ------------------------------------ |
| Set up a provider, client and model | `aigw setup`                         | Follow **Next**                      |
| Apply a reviewed team file          | `aigw setup --from team.toml`        | Connect any one provider when ready  |
| Inspect current state               | `aigw`                               | Follow **Next**                      |
| Point a client at a model           | `aigw use <client> <provider/model>` | Follow **Next**                      |
| Replace one provider's Token        | `aigw provider rotate <provider>`    | Follow **Next**                      |
| Diagnose a problem                  | `aigw check --depth local --verbose` | Run its **Fix**                      |
| Find a model to use                 | `aigw model search <query>`          | `aigw use <client> <provider/model>` |

`aigw check` sends one bounded inference request by default and may use
provider quota. Use `aigw check --depth endpoint` for a model-free endpoint
check.

## Install

On macOS, install the signed and notarized stable release with Homebrew:

```bash
brew install --cask HengYangDS/tap/aigw
```

For macOS, Linux, or Windows without Homebrew, download the archive and checksum
manifest for the host from either release peer. The published matrix covers
Darwin, Linux, and Windows on AMD64 and ARM64. Extract the archive, then run:

```bash
./aigw self install
```

```powershell
.\aigw.exe self install
```

The portable installer copies only the running executable, retains one
predecessor for rollback, and prints the installed path. It does not alter
`PATH`, read Tokens, configure clients, or start another product. Confirm the
active program before setup:

```bash
aigw self status
aigw version
```

If the installed directory is not yet on `PATH`, invoke the printed absolute
path.

Package-manager and portable installations have different owners; see the
[installation lifecycle](docs/concepts/product-concepts.md#installation-lifecycle)
before updating or removing one.

## Upgrade from 0.6.1 or earlier

0.7.0 reads only `aigw.toml`. It never reads or changes AIGW's `config.toml` or
the component files `config.*.toml` that earlier releases wrote, and it ships no
importer for them. A home that holds only those files has no configuration, and
`aigw` names `aigw setup`.

After upgrading:

1. Run `aigw setup --from <team file>` or `aigw setup`. A provider that you add
   under its earlier ID finds the Token already stored for it, and setup reuses
   that Token without a prompt.
2. Run `aigw use <client> <provider/model>` for each client.

Client sessions that started before the upgrade keep working, because they call
the credential reader they started with, and that reader still reads the
untouched `config.toml`. Delete `config.toml` and `config.*.toml` once those
sessions have ended; AIGW never deletes them. A Hermes session that started
before the upgrade names provider IDs that no longer exist, because Hermes
provider IDs are now `aigw-<provider>-<protocol>`, such as
`aigw-dmxapi-responses`. `aigw self rollback` cannot return to a release older
than 0.7.0; see [Update and rollback](#update-and-rollback). A team member who
still runs 0.6.1 cannot read a version 8 team file, so everyone upgrades first.

## Add a provider

Interactive setup adds a provider or chooses an existing one, then asks for an
installed client and a model from the provider's catalog, saves them, and
checks the client. It is safe to run again:

```bash
aigw setup
```

The guided flow asks for a protocol when a client speaks more than one that
the provider serves. Without an installed client that can use the provider,
it adds the provider and stops; install a client, then run `aigw setup` again
to choose it and a model. Guided setup enables the client it configures, and
it is the one write without a preview: `aigw setup --from team.toml --dry-run`
previews a team file instead. `aigw check` checks the client files that AIGW
wrote, not the act of installing a client.

A script adds a provider and points a client at one of its models in two
commands. `aigw provider add` stores the Token only after the provider accepts
it in one authenticated request that names no model:

```bash
printf '%s\n' "$TEAM_TOKEN" \
  | aigw provider add team \
      --label "Team endpoint" \
      --endpoint responses=https://api.example.com/v1 \
      --token-stdin
aigw use codex team/model-id
```

`aigw use` adds a model that the provider's catalog lists; `--unlisted` adds
one that it does not. Use `--endpoint anthropic=<url>` and an
Anthropic-compatible client for a native Anthropic endpoint. AIGW records the
endpoint choice; it does not insert or manage a gateway.

A team can instead distribute a reviewed, token-free team file. Save the
tracked [`manifests/team.toml`](manifests/team.toml) as a local `team.toml`,
then run:

```bash
aigw setup --from team.toml
```

Applying the team file requires neither every provider Token nor an installed
client. To connect one provider during setup, name it. Its Token must be
available: a writable backend prompts for it, while `env` mode reads
`AIGW_TOKEN_DMXAPI` from the invoking process:

```bash
aigw setup --from team.toml --provider dmxapi
```

With a writable backend, unattended setup can pass that provider's Token on
standard input:

```bash
printf '%s\n' "$DMXAPI_TOKEN" \
  | aigw setup --from team.toml --provider dmxapi --token-stdin
```

Other providers remain available without becoming prerequisites. With the
environment backend, set one [compatible provider variable](docs/guides/team-rollout.md#automated-rollout),
then activate its unselected reviewed recommendations without repeating setup:

```bash
aigw sync
aigw check
```

With a writable credential backend, run `aigw provider rotate <provider>`, then
point a client at a model with `aigw use <client> <provider/model>`; `use`
synchronizes that client. An existing explicit selection is never replaced by
a newly available recommendation.

`<client>` is `claude`, `claude-desktop`, `codex`, or `hermes`. A model that a
client could use on more than one protocol needs an explicit `--protocol`; see
[Hermes protocol and model selection](docs/guides/team-rollout.md#hermes-protocol-and-model-selection).

`sync` discovers installed clients and changes only AIGW's settings in their
files. It reconciles enabled clients whose model, authentication, and client
files are available. With the read-only environment backend, a newly available
provider Token can also activate an unselected client's reviewed
recommendation; writable stores are not searched for unused Tokens. `sync`
never replaces an explicit selection or Token, or creates a missing client.
See the complete [first-member and deferred-client journey](docs/guides/team-rollout.md#new-member).

### Credential storage

Automatic selection uses the platform credential service when available and a
single platform-local fallback otherwise. An explicit
`AIGW_SECRET_BACKEND=keyring`, `file`, or `env` choice fails closed instead of
searching another store.

Environment mode is read-only and portable. Supply only the providers used by
that process through `AIGW_TOKEN_<PROVIDER>`; the client process must inherit
the same environment when its credential helper reads the Token. Exact
variable encoding, native-store behavior, and external credential executables
belong to the [security model](docs/architecture/security-model.md#credential-storage).

## Use it every day

```bash
aigw
aigw use <client> <provider/model>
aigw status
aigw check
aigw check <client> --depth client
```

| Command           | Contract                                                                       |
| ----------------- | ------------------------------------------------------------------------------ |
| `status`          | Show what each client uses and whether it is ready; no network, no Token read  |
| `check`           | Check that each client works, as deep as `--depth` asks; `model` may use quota |
| `model list`      | List the models you have added; no network, no Token read                      |
| `model search`    | Search what providers offer, from their authenticated catalogs                 |
| `sync`            | Make client files match the configuration                                      |
| `config rollback` | Restore the configuration and client files from a verified snapshot or backup  |

`aigw check` takes four depths, and each runs the ones before it: `local`
checks the configuration, the Token's presence, the client files and the
environment without a request; `endpoint` sends one authenticated request
that names no model; `model`, the default, sends one bounded inference
request; `client` runs a real client session, which may make several
quota-consuming requests. It checks every enabled client unless you name
clients. Use `aigw sync --dry-run --json` before reconciling drift. Human
output gives one next action; machine consumers use the command's JSON mode.

### Codex connection retries

Declare native retry budgets explicitly when an endpoint owns recovery:

```bash
aigw client edit codex --request-retries 0 --stream-retries 0 --dry-run --json
aigw client edit codex --request-retries 0 --stream-retries 0
aigw client edit codex --native-retries
```

Zero disables that native retry budget; `--native-retries` removes both, so
Codex uses its own defaults. Retry budgets belong to the client's provider:
changing the model or protocol within that provider keeps them, and
`aigw use` with another provider's model clears them and says so. Only Codex
currently exposes admitted native budgets. AIGW writes the declaration; it
neither retries requests nor guarantees exactly-once inference.

The client's selection owns this policy, not the shared team file. A portable
program rollback first gives the retained program the current configuration,
and refuses a program that cannot read it without changing programs or
credentials. A Homebrew rollback prepares the Keychain items for the earlier
release whatever the policy and does not refuse because retry budgets are set.

### Client forwarding

Send one client through a local endpoint, such as the Responses Proxy, while
its provider and model stay the same:

```bash
aigw client edit <client> --via http://127.0.0.1:8792/v1 --dry-run --json
aigw client edit <client> --via http://127.0.0.1:8792/v1
aigw client edit <client> --direct
```

The client's selection records the upstream that forwarding reaches. AIGW
clears forwarding, and says so, whenever the client's upstream endpoint
changes: `aigw use` with another provider or protocol, or
`aigw provider edit --endpoint` for the endpoint that the client reaches.
`--direct` drops the forwarding endpoint and its recorded upstream together.

## Product model

| Noun     | Owns                                                                                  |
| -------- | ------------------------------------------------------------------------------------- |
| Provider | A service you hold a Token for: its endpoints, one per protocol, and the Token        |
| Model    | What a provider serves, named `provider/model`, with the protocols AIGW may use it on |
| Family   | Optional: which canonical model a provider's model is                                 |
| Client   | `claude`, `claude-desktop`, `codex` or `hermes`, with its selected model and files    |

There is no global model selection: `aigw use <client> <provider/model>`
changes exactly one client. A present Token, synchronized files, an endpoint
check and a successful client session are different readiness claims.

Current source Adapters support Codex CLI/Desktop through their shared Codex
Home, Claude Code through its user settings, Hermes through its provider
configuration, and Claude Desktop through its separate third-party inference
library. Tokens remain behind credential helpers. Missing clients stay
untouched, and release support remains limited to clients with completed native
evidence. Future clients require a separately admitted Adapter rather than
reuse of another client's state.

See [Product concepts](docs/concepts/product-concepts.md) for the vocabulary
and [Adapter admission](docs/governance/adapter-admission.md) for extension
requirements.

## Boundaries

| Surface                                                              | Owner                  |
| -------------------------------------------------------------------- | ---------------------- |
| Providers, Tokens, models and each client's selection                | AIGW                   |
| AIGW-marked settings in Codex, Claude Code, Hermes and Desktop files | AIGW                   |
| Codex conversations, JSONL, SQLite, and per-conversation models      | Codex                  |
| Claude sessions and unrelated settings                               | Claude Code            |
| External gateways and compatibility services                         | Their product/operator |
| IDE and ACP configuration                                            | The IDE or ACP product |

A provider may use a direct URL or an independently operated compatible
endpoint, including an explicit loopback URL. AIGW stores that choice and
writes it into client files; it never installs, starts, stops, or diagnoses the
external service.

Adding a compatible endpoint or model is configuration. A new host surface is a
Client Adapter. Incompatible wire behavior belongs in an independent data plane.
See the [extension model](docs/architecture/authority-and-projection-boundary.md#extension-model).

## Update and rollback

`aigw config rollback` restores the configuration and client files.
`aigw self rollback` restores the retained portable program.
Homebrew-managed installations use Homebrew for program upgrades and removal.

For a portable installation:

```bash
aigw self update
aigw sync
aigw check
```

To restore the retained predecessor:

```bash
aigw self rollback
aigw sync
aigw check
```

Program rollback first gives the retained program an isolated copy of the
current configuration and, when that program cannot read it, refuses and
changes nothing. A release older than 0.7.0 cannot read `aigw.toml`, so
`aigw self rollback` cannot return to one. An earlier release that you install
by other means finds its own files as they were before the upgrade, without
later changes.

On macOS, the installed AIGW program moves Keychain Tokens saved by 0.3.5 and
earlier to its own reader the first time it uses them; nothing needs to be
entered again. The versioned credential readers that clients call never move a
Token, so a predecessor that is still installed keeps reading it.
`aigw self rollback` returns the Tokens to the retained program's reader after
admitting the rollback and before activating that program. Before a Homebrew
downgrade to a release of 0.7.0 or later, run `aigw self rollback` on the
Homebrew installation: it returns the Tokens without touching Homebrew's files,
and the next Token use by a kept release takes them back. If it cannot read the
configuration, it moves no Token and reports that the credentials are not ready.
Without that step, the older program asks macOS to allow access to each Token.

The first Homebrew upgrade from 0.3.5 or earlier is a managed cutover, not a bare
`brew upgrade aigw`: existing sessions may still call Homebrew's public
credential-command link. Before the switch, the release operator must stage
the final successor's private reader, verify its exact commands against the
existing Keychain items without a prompt, write the managed clients' files
ahead of the switch, and prefetch the verified Cask. Measure the bounded link
gap, then immediately test captured commands and real clients; restore the
predecessor if acceptance fails. Do not switch if a required existing native
item cannot be read noninteractively.

A session still caching the public link may fail once during the gap. Blanket
client shutdown or restart does not eliminate that risk and is not an upgrade
step. Later versioned-reader upgrades must keep old commands callable. See the
[credential succession decision](docs/decisions/dr-0011-single-portable-token-backend.md#consequences-and-boundary).

`aigw self uninstall` removes AIGW's settings from the client files, then the
portable executable and its predecessor. It retains versioned credential
reader files for cached or shared callers; a client whose settings are removed
cannot read its Token merely because the executable remains. Providers,
models, client selections, Tokens, configuration backup, and user-authored
client state remain.

For a Homebrew installation, disable enabled clients first with
`aigw client disable <client>` and let Homebrew remove the package; AIGW does
not overwrite or delete package-manager-owned files.

Online portable updates read the GitHub release that official builds name; a
GitLab peer joins only when `AIGW_GITLAB_RELEASE_ORIGIN` and
`AIGW_GITLAB_RELEASE_REPOSITORY` name one at run time. A selected peer must
provide one complete release; AIGW never combines a tag, checksum, or asset
across peers. Online portable updates authenticate `checksums.txt.sig` against
the release key embedded in AIGW before checking hashes, extracting, or executing
downloaded programs. Missing or invalid signatures stop the update and preserve
the current program and rollback file; downloads never supply their own trust.
Maintainer publication belongs to
[Forge operations](docs/operations/forge-operations.md).

## Contribute

```bash
mise run bootstrap
mise run check
mise run native
```

Use an isolated Work Lane, start behavior changes with a failing regression, run
focused checks before the complete graph, and keep generated output under its
declared owner. The [contribution workflow](CONTRIBUTING.md) maps each invariant
to its source, test, gate, and evidence boundary.

## Documentation

- [Documentation index](docs/README.md)
- [Architecture and projection boundary](docs/architecture/authority-and-projection-boundary.md)
- [Security model](docs/architecture/security-model.md)
- [Team rollout](docs/guides/team-rollout.md)
- [Terminal experience](docs/experience/terminal-experience.md)
- [Change and release policy](docs/governance/change-and-release-policy.md)
- [Decision Records](docs/decisions/README.md)
- [Competitive research](docs/research/provider-tooling-assessment.md)

Licensed under the [MIT License](LICENSE).
