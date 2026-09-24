# AIGW CLI

AIGW is a local-first control plane for teams that use reviewed third-party AI
services. It manages Accounts, Tokens, reusable Routes, explicit client
bindings, and guarded native projections. It does not relay model traffic, run
a gateway, or own conversation state.

Each client resolves its endpoint and model from its own binding. Read the
[authority boundary](docs/architecture/authority-and-projection-boundary.md) for
the complete design.

## Start here

| Goal                          | Command                           | Then                               |
| ----------------------------- | --------------------------------- | ---------------------------------- |
| Connect the first Account     | `aigw setup`                      | `aigw check`                       |
| Import reviewed team settings | `aigw setup --from team.toml`     | Connect any one Account when ready |
| Inspect current state         | `aigw`                            | Follow **Next**                    |
| Bind a Route to a client      | `aigw use --for <client> <route>` | `aigw check`                       |
| Replace one Account Token     | `aigw rotate <account>`           | `aigw check`                       |
| Diagnose a problem            | `aigw doctor`                     | Run its recommended action         |
| Migrate retained local state  | `aigw config migrate --dry-run`   | Review, then apply                 |

## Install

On macOS, install the signed and notarized stable release with Homebrew:

```bash
brew install --cask HengYangDS/tap/aigw
```

For macOS, Linux, or Windows without Homebrew, download the archive and checksum
manifest for the host from either release peer. The published matrix covers
Darwin, Linux, and Windows on AMD64 and ARM64. Extract the archive, then run:

```bash
./aigw install
```

```powershell
.\aigw.exe install
```

The portable installer copies only the running executable, retains one
predecessor for rollback, and prints the installed path. It does not alter
`PATH`, read Tokens, configure clients, or start another product. Confirm the
active program before setup:

```bash
aigw installation
aigw --version
```

If the installed directory is not yet on `PATH`, invoke the printed absolute
path.

Package-manager and portable installations have different owners; see the
[installation lifecycle](docs/concepts/product-concepts.md#installation-lifecycle)
before updating or removing one.

## Connect an Account

Interactive setup creates one Account and one Route, then binds the explicitly
selected client when its prerequisites are available:

```bash
aigw setup
```

To connect a direct OpenAI Responses endpoint without a team manifest:

```bash
printf '%s\n' "$TEAM_TOKEN" \
  | aigw add team \
      --label "Team endpoint" \
      --openai-url https://api.example.com/v1 \
      --model model-id \
      --for codex \
      --token-stdin
```

Use `--anthropic-url` and an Anthropic-compatible client for a native Anthropic
endpoint. AIGW records the endpoint choice; it does not insert or manage a
gateway.

A team can instead distribute a reviewed, token-free manifest:

```bash
aigw setup --from team.toml
```

The tracked [`manifests/team.toml`](manifests/team.toml) is the current team
catalogue. Importing it requires neither every provider Token nor an installed
client. To connect one Account during setup, name only that Account:

```bash
aigw setup --from team.toml --account dmxapi
```

For unattended setup, bind the stdin Token to the same Account explicitly:

```bash
printf '%s\n' "$DMXAPI_TOKEN" \
  | aigw setup --from team.toml --account dmxapi --token-stdin
```

Other Accounts remain available without becoming prerequisites. If a Token or
supported client arrives later, use the existing configuration rather than
repeating setup:

```bash
aigw rotate dmxapi
aigw use --for codex dmxapi-gpt-6-astra
aigw sync
aigw check
```

`sync` discovers installed clients and changes only AIGW-owned projection state.
It reconciles enabled client bindings whose Route, authentication, and native
surface are available. It never selects a Route, enables an unbound client,
replaces Tokens, or creates missing clients. See the complete
[first-member and deferred-client journey](docs/guides/team-rollout.md#new-member).

An installation retaining the preceding schema fails normal commands closed.
Preview its deterministic replacement before applying it:

```bash
aigw config migrate --dry-run
aigw config migrate
aigw sync
aigw check
```

Migration changes only AIGW configuration. It preserves Account Tokens, client
files, sessions, and one exact predecessor configuration. Before rolling the
program back, restore that predecessor with `aigw config migrate --rollback`.

### Credential storage

Automatic selection uses the platform credential service when available and a
single platform-local fallback otherwise. An explicit
`AIGW_SECRET_BACKEND=keyring`, `file`, or `env` choice fails closed instead of
searching another store.

Environment mode is read-only and portable. Supply only the Accounts used by
that process through `AIGW_TOKEN_<ACCOUNT>`; the client process must inherit the
same environment when its projected helper reads the Token. Exact variable
encoding, native-store behavior, and external credential executables belong to
the [security model](docs/architecture/security-model.md#credential-storage).

## Use it every day

```bash
aigw
aigw use --for <client> <route>
aigw status
aigw check
aigw verify --for <client>
```

| Command    | Contract                                                                    |
| ---------- | --------------------------------------------------------------------------- |
| `status`   | Observe Client Bindings and projection readiness without reading Tokens     |
| `check`    | Check credentials and projections, then selected-model inference by default |
| `doctor`   | Explain current problems without mutation                                   |
| `repair`   | Reconcile bounded AIGW-owned client state                                   |
| `test`     | Test an endpoint without proving native-client behavior                     |
| `verify`   | Run one explicit native-client request that may consume quota               |
| `rollback` | Restore a verified AIGW configuration checkpoint                            |

Use `aigw repair --dry-run --json` before repairing drift. Human output gives one
next action; machine consumers use the command's JSON mode where available.

## Product model

| Entity            | Owns                                                      |
| ----------------- | --------------------------------------------------------- |
| Account           | Provider endpoints and one logical Token boundary         |
| Route             | `account + model` and reusable display metadata           |
| Client binding    | One client's Route, enabled intent, protocol, and options |
| Native projection | The AIGW-owned portion of one client's configuration      |

There is no global model selection: `aigw use --for <client> <route>` changes
exactly one client binding. A present Token, synchronized file, endpoint probe,
and successful native-client request are different readiness claims.

Current source Adapters support Codex CLI/Desktop through their shared Codex
Home, Claude Code through its user settings, Hermes through its provider
configuration, and Claude Desktop through its separate third-party inference
library. Account Tokens remain behind credential helpers. Missing clients stay
untouched, and release support remains limited to clients with completed native
evidence. Future clients require a separately admitted Adapter rather than
reuse of another client's state.

See [Product concepts](docs/concepts/product-concepts.md) for the entity model
and [Adapter admission](docs/governance/adapter-admission.md) for extension
requirements.

## Boundaries

| Surface                                                         | Owner                  |
| --------------------------------------------------------------- | ---------------------- |
| Accounts, Tokens, Routes, and client bindings                   | AIGW                   |
| AIGW-marked Codex, Claude Code, Hermes, and Desktop projections | AIGW                   |
| Codex conversations, JSONL, SQLite, and per-conversation models | Codex                  |
| Claude sessions and unrelated settings                          | Claude Code            |
| External gateways and compatibility services                    | Their product/operator |
| IDE and ACP configuration                                       | The IDE or ACP product |

An Account may use a direct provider URL or an independently operated compatible
endpoint, including an explicit loopback URL. AIGW stores and projects that
choice; it never installs, starts, stops, or diagnoses the external service.

Adding a compatible endpoint or model is configuration. A new host surface is a
Client Adapter. Incompatible wire behavior belongs in an independent data plane.
See the [extension model](docs/architecture/authority-and-projection-boundary.md#extension-model).

## Update and rollback

For a portable installation:

```bash
aigw update
aigw sync
aigw check
```

To restore the retained predecessor:

```bash
aigw update --rollback
aigw sync
aigw check
```

Program rollback does not silently downgrade configuration. If the predecessor
cannot read an isolated copy of current configuration, AIGW leaves both program
and configuration unchanged. Restore a compatible configuration explicitly with
`aigw config migrate --rollback`, then retry `aigw update --rollback`.

`aigw uninstall` withdraws AIGW-owned client projections and removes the
portable executable plus its predecessor. It preserves Accounts, Routes,
Client Bindings, Tokens, configuration backup, and user-authored client state.

For a Homebrew installation, disable enabled clients first and let Homebrew
remove the package; AIGW does not overwrite or delete package-manager-owned
files.

Updates resolve GitLab and GitHub as independent optional peers. A selected peer
must provide one complete release; AIGW never combines a tag, checksum, or asset
across peers. Maintainer publication belongs to
[Forge operations](docs/operations/forge-operations.md).

## Contribute

```bash
mise install --locked
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
- [Decision register](docs/decisions/decision-register.md)
- [Competitive research](docs/research/provider-tooling-assessment.md)

Licensed under the [MIT License](LICENSE).
