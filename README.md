# AIGW CLI

A local-first control plane for teams using reviewed third-party AI services.

AIGW manages Accounts, Tokens, Profiles, Routes, and native client projections.
It does **not** relay model traffic, run a gateway, or own conversation state.

Codex and Claude Code call their selected endpoints directly. See the
[product position](docs/architecture/authority-and-projection-boundary.md#product-position)
for the configuration and traffic boundaries.

## Start here

| Goal                          | Command                       | Next step                           |
| ----------------------------- | ----------------------------- | ----------------------------------- |
| Connect the first service     | `aigw setup`                  | `aigw check`                        |
| Inspect the active selection  | `aigw`                        | Follow **Next**                     |
| Select a profile              | `aigw use <profile>`          | `aigw check`                        |
| Replace an Account Token      | `aigw rotate <account>`       | `aigw check`                        |
| Diagnose local integration    | `aigw doctor`                 | Run its recommended action          |
| Import reviewed team settings | `aigw setup --from team.toml` | Connect any one Account when needed |

Advanced object management remains under explicit command groups.

## Install

Install the checksum-verified archive matching the host from either independent
release plane: `darwin_amd64`, `darwin_arm64`, `linux_amd64`, `linux_arm64`,
`windows_amd64`, or `windows_arm64`.

A portable archive contains only the executable, README, and license. Run the
executable once to install it in the platform's user program directory:

```bash
./aigw install
```

```powershell
.\aigw.exe install
```

The default is `~/.local/bin/aigw` on macOS and Linux, and
`%LOCALAPPDATA%\Programs\aigw\bin\aigw.exe` on Windows. Windows falls back to
`%APPDATA%` when `LOCALAPPDATA` is unset. The installer prints the exact path.
An explicit destination is available for isolated use:

```bash
./aigw install --target /path/to/aigw
```

```powershell
.\aigw.exe install --target C:\path\to\aigw.exe
```

Installation does not change `PATH`. Until its directory is on your shell's
search path, invoke the installed executable directly; running `aigw` by name
may fail or select another installation. For the default destinations:

```bash
"$HOME/.local/bin/aigw" --version
"$HOME/.local/bin/aigw" setup
```

```powershell
$programRoot = if ($env:LOCALAPPDATA) { $env:LOCALAPPDATA } else { $env:APPDATA }
$aigw = Join-Path $programRoot 'Programs\aigw\bin\aigw.exe'
& $aigw --version
& $aigw setup
```

Use the printed path instead for a custom destination. Add its directory through
your shell or operating system's normal user `PATH` settings if you want the
short command. Then verify `aigw installation` resolves to that installation
before following the commands below. A team import can replace interactive
`setup` with `setup --from team.toml` at either installed path.

`aigw install` copies only the running executable and retains one predecessor
for rollback. Reinstalling identical bytes preserves the existing program and
its predecessor; a fresh installation has no predecessor. It does not edit
shell startup files, retrieve a release, store
credentials, configure clients, or start another product. `aigw uninstall` first withdraws every AIGW-owned client projection, then
removes the installed executable and that rollback copy. Accounts, Profiles,
Routes, Tokens, user-authored client settings, and the explicit configuration
backup remain intact.

Portable lifecycle commands do not manage Homebrew installations. When the
resolved executable or destination belongs to a Homebrew receipt, AIGW stops
before downloading, replacing files, or withdrawing client projections. Use
Homebrew to manage that installation; copying another executable over its files
would bypass its package inventory. This ownership guard does not imply that an
AIGW Homebrew package has been published.

Run `aigw installation` to inspect the invoked command, actual program file and
retained predecessor. `aigw installation --json` provides a schema-versioned
description with paths, byte counts and SHA-256 digests without requiring this
checkout or valid Account configuration. A missing predecessor is `null`;
observed bytes do not establish release trust or successful rollback.

GitLab and GitHub publish independently. Either release plane may supply a
verified installation. When both are reachable during update, AIGW requires
their version and current-platform asset bytes to agree; it never combines
assets from different Forges.

## Connect a service

Interactive setup creates one Account, one Profile, one Route, and one local
Token slot:

```bash
aigw setup
```

A team can distribute a reviewed, token-free manifest:

```bash
aigw setup --from team.toml
```

Start from [`manifests/team.toml`](manifests/team.toml). It contains the
reviewed team Accounts, Profiles, and recommended Routes, but no Token or
workstation-specific client path. Import does not require a Token or an
installed client, so the same file works for a new workstation and for a
machine where Claude Code or Codex will be installed later.

To connect one Account while importing, name its manifest ID. Other Accounts
remain available without becoming setup requirements:

```bash
aigw setup --from team.toml --account dmxapi
```

For non-interactive automation, one stdin Token must have one explicit owner:

```bash
printf '%s\n' "$DMXAPI_TOKEN" \
  | aigw setup --from team.toml --account dmxapi --token-stdin
```

All `--token-stdin` commands read through EOF, up to 64 KiB. Supply one non-empty
visible ASCII Token, optionally followed by one LF or CRLF; embedded whitespace,
extra lines and control characters are rejected rather than trimmed or ignored.
Setup stores the credential. To test connectivity without storing it, use
`aigw test --profile <profile> --token-stdin`; its optional `--config` accepts an
absolute AIGW configuration path without changing the default configuration.
This tests the endpoint's HTTP response, not model inference or client behavior.

If the catalogue was imported without a Token, connect an Account later and
select any of its Profiles:

```bash
aigw rotate dmxapi
aigw use dmxapi-gpt-5.6-sol
```

If a supported client is installed after setup, `aigw sync` discovers it and
creates only AIGW-owned projection state. Synchronization does not ask for or
replace a Token or write client-owned credentials. Verify the projected route
through the installed client when it is ready:

```bash
aigw sync
aigw check
aigw verify --for codex
```

`aigw status` reports selected Routes and local projection readiness without
reading Token values or invoking clients. `aigw check` adds endpoint checks;
`aigw verify --for <client>` runs the selected native client. Neither a present
Token nor a synchronized file alone proves a working client request.

### Environment variables

Interactive users do not need environment variables. Without an override,
AIGW observes the platform credential service's availability without reading a
Token. If it is unavailable, AIGW selects one fallback beneath its data directory:
an owner-only store on macOS and Linux, or a Windows DPAPI-protected store.
The first credential mutation persists the automatic choice before changing a
Token. Read-only commands and credential reads do not create selection state;
AIGW never searches multiple stores for the same Token.

See [Credential storage](docs/architecture/security-model.md#credential-storage)
for the exact macOS, Linux, Windows, and environment-backed contracts.

For an explicit choice, set `AIGW_SECRET_BACKEND`:

| Value     | Source                               |
| --------- | ------------------------------------ |
| `keyring` | Native platform credential service   |
| `file`    | Platform-local store described above |
| `env`     | Environment of the calling process   |

An explicit choice does not fall back to another backend. Environment mode is
read-only: AIGW does not persist, rotate, or delete those Tokens.

In environment mode, `AIGW_TOKEN_<ACCOUNT>` supplies one Account's API Token.
Uppercase the Account ID and encode punctuation as ASCII hex: `-` becomes
`_2D`, `.` becomes `_2E`, and `_` becomes `_5F`. For example, `dmx-api` uses
`AIGW_TOKEN_DMX_2DAPI`.

Optional platform diagnostics use a separate pair:
`AIGW_DIAGNOSTIC_SYSTEM_TOKEN_<ACCOUNT>` and
`AIGW_DIAGNOSTIC_USER_ID_<ACCOUNT>`. Both must be present; neither replaces the
API Token. These variables use the same Account encoding and `env` backend.

The client process must inherit the selected backend and Token variables too:
its credential helper runs in that process environment. Setting a variable in
one terminal does not configure an already-running client or a desktop launcher.
Use a persistent backend for clients that do not inherit that environment.

Output accessibility is independent: `AIGW_ACCESSIBLE=1` selects
accessibility-oriented terminal output. Update-source variables belong to
[release selection](#release-sources), not service setup.

## Use it every day

```bash
aigw
aigw use <profile>
aigw check
aigw doctor
aigw repair --dry-run --json
aigw repair
aigw rotate [account]
```

| Command    | Purpose                                                               |
| ---------- | --------------------------------------------------------------------- |
| `status`   | Show selected Routes, local projection readiness, and one next action |
| `check`    | Verify configuration, client projection, and endpoint passage         |
| `doctor`   | Explain a problem without mutation                                    |
| `repair`   | Reconcile bounded AIGW-owned client state                             |
| `test`     | Test configured connectivity and authentication                       |
| `verify`   | Make an explicit minimal model request that may consume quota         |
| `rollback` | Restore AIGW-managed configuration only                               |

Human output is task-oriented and terminal-width aware. Automation uses stable
JSON flags where available. Expected failures do not emit tracebacks, warning
dumps, or unrelated usage text.

## Product model

| Entity  | Owns                                                                      | Does not own           |
| ------- | ------------------------------------------------------------------------- | ---------------------- |
| Account | Provider endpoints and one logical Token boundary                         | Client selection       |
| Profile | `account + client + model` and an optional Codex-native provider identity | Endpoint credentials   |
| Route   | One client's explicit Profile selection                                   | Provider fallback      |
| Adapter | One native client projection                                              | Another client's state |

See [Product concepts](docs/concepts/product-concepts.md) for entity relationships.

The current admitted clients are:

- **Codex CLI and Codex Desktop**, which share one Codex Home;
- **Claude Code**, configured through its official user settings and credential-helper boundary.

Future clients require a new admitted adapter. They are not inferred from a
provider name and are not configured by the current release.

## Ownership boundaries

| Surface                                                          | Owner                    |
| ---------------------------------------------------------------- | ------------------------ |
| Account metadata, Tokens, Profiles, Routes                       | AIGW                     |
| AIGW-marked Codex provider/model projection                      | AIGW                     |
| AIGW-owned Claude Code endpoint/model keys and credential helper | AIGW                     |
| Codex conversations, JSONL, SQLite, model metadata               | Codex                    |
| Claude session behavior                                          | Claude Code              |
| External gateway or compatibility process                        | Its own product/operator |
| IDE and ACP configuration                                        | The IDE or ACP product   |

AIGW never edits Codex history or Desktop-only GUI state. A loopback endpoint is
an ordinary Account endpoint; AIGW does not start, stop, configure, or diagnose
the process listening there.

### Native Codex providers

Most Codex Profiles omit `model_provider` and use AIGW's canonical provider
projection. When an endpoint requires its own Codex-native provider identity,
declare it on that Codex Profile:

```toml
[profiles.aws-codex]
label = "AWS Codex"
account = "aws"
client = "codex"
model = "openai.gpt-5.6-sol"
model_provider = "amazon-bedrock"
```

For Account-Token authentication, the default helper is the absolute AIGW
executable with `credential codex <projection-fingerprint>`. The fingerprint
matches the projected client, Account and endpoint before the helper reads a
Token; it is not a credential or caller authorization. A retained, stale
projection requires `aigw sync` and a client configuration reload. Token rotation
is observed on the next helper invocation; the client controls its refresh
timing. A Profile using `authentication = "client-native"` leaves authentication
to Codex and receives no AIGW Token helper. Provider naming does not select
authentication ownership. Neither mode installs a proxy or changes conversation
state.

An operator can select another trusted credential executable in the local
`[adapters.codex]` or `[adapters.claude]` table with `credential_command`.
This field is one absolute executable path, not a shell command or a Token.
The helper must implement `credential <client> <projection-fingerprint>`;
its only successful stdout is the requested Token. This host-local setting
does not belong in the team manifest. See the
[external credential boundary](docs/architecture/security-model.md#external-credential-executable)
before enabling it, then review `aigw sync --dry-run --json` and apply `aigw sync`.
Sync and program upgrades preserve the explicit choice. Disabling a client
removes its projection but retains this policy; later sync leaves it disabled
until explicit `aigw adapter enable`. Full uninstall removes Adapter policy,
not the external executable or its credentials.

## Team rollout

Import the reviewed manifest first; add any one usable Account Token and install
a client when ready. Missing credentials or clients remain deferred, not fatal.

Use [Team rollout](docs/guides/team-rollout.md) for manifest review, staged
adoption, and rollback. Tokens never enter the manifest or repository.

## Update and rollback

```bash
aigw update
aigw sync
aigw check
```

To return to the retained program, keep client integrations enabled and run:

```bash
aigw update --rollback
aigw sync
aigw check
```

Every installation uses the same portable lifecycle and retains one immediate
predecessor. A verified offline candidate may be supplied explicitly with its
checksum manifest; source trees, loose binaries, tags, and self-authored
checksums are not installation evidence.

Program replacement preserves the configured Accounts, Profiles, Routes and
enabled client locations. Run `sync` with the newly active executable after
either replacement, then check readiness before resuming client work. Use
`aigw verify --for <client>` when a real client request is needed; it may consume
quota. Configuration readability and successful synchronization do not prove
that every client or Provider supports a particular predecessor.

A program rollback does not convert configuration to an older schema. Before replacing the program,
AIGW checks whether the actual predecessor can read an isolated copy of the
current configuration. An incompatible rollback leaves both programs and the
configuration unchanged. Restore a compatible configuration explicitly with
`aigw rollback` (or `aigw rollback --last-change` when that backup is the intended
compatible state), then retry program rollback. If no compatible recovery source
exists, keep the current program; AIGW does not delete newer settings to make an
older program appear usable.

### Release sources

Built-in release coordinates are the default. To select another source, supply
both variables for that Forge; AIGW never combines a partial override with
built-in coordinates:

- GitHub: `AIGW_GITHUB_RELEASE_ORIGIN` and
  `AIGW_GITHUB_RELEASE_REPOSITORY` select an origin and `owner/repository`.
- GitLab: `AIGW_GITLAB_RELEASE_ORIGIN` and
  `AIGW_GITLAB_RELEASE_REPOSITORY` select an origin and `namespace/project`.

Build-time coordinates and runtime overrides follow one address policy. Public
hosts require HTTPS; explicit private, link-local and loopback IP addresses,
`localhost` and reserved `.test` hosts also admit HTTP. HTTP does not encrypt
traffic: prefer HTTPS even on private networks. Artifact integrity still requires
the trusted release signature; an admitted address is not authentication.

Private GitHub lookup checks `AIGW_GITHUB_TOKEN`, `GITHUB_TOKEN`, then `GH_TOKEN`,
without persisting them. Private GitLab lookup uses `glab` credentials or falls
back to `GITLAB_TOKEN`; that fallback requires an explicit HTTPS GitLab origin.
These credentials authorize release downloads, not model requests. Repository
maintainers can find publication instructions in [CONTRIBUTING](CONTRIBUTING.md).

## Verify a source checkout

```bash
mise install --locked
mise run bootstrap
mise run check
mise run native
mise exec --locked -- go run ./tools/forge commits --email '<product author email>' --allowed-signers '<path>'
mise exec --locked -- go run ./tools/forge tags --allowed-signers '<path>'
```

These tasks are the portable development entrypoints. `bootstrap`
reconstructs this Work Lane's locked repository dependencies; `check` runs the
complete source and governance gate; `native` proves the current host.
The separate [`release` task](CONTRIBUTING.md#signed-artifact-builds) builds
deterministic artifacts in `dist` without publishing them. It requires explicit
artifact-signing inputs; ordinary source and native acceptance do not.

## Documentation

| Need                                | Source of truth                                                                                             |
| ----------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| Concepts                            | [Account, Profile, Route, Adapter](docs/concepts/product-concepts.md)                                       |
| Client and control-plane boundaries | [Architecture](docs/architecture/authority-and-projection-boundary.md)                                      |
| Human terminal behavior             | [Terminal experience](docs/experience/terminal-experience.md)                                               |
| Security                            | [Security model](docs/architecture/security-model.md)                                                       |
| Team adoption                       | [Team rollout](docs/guides/team-rollout.md)                                                                 |
| Release evidence                    | [Quality and platform evidence](docs/governance/change-and-release-policy.md#quality-and-platform-evidence) |
| Development                         | [CONTRIBUTING](CONTRIBUTING.md)                                                                             |
| Full index                          | [Documentation root](docs/README.md)                                                                        |

Licensed under the MIT License: [MIT](LICENSE).
