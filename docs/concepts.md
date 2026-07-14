# Concepts

## Account

An Account is one upstream provider account boundary: display label, supported protocol endpoints, optional exact-diagnostics declaration, and exactly one logical Token. The secret is stored at `AIGW_TOKEN/<account>` in the operating-system credential store; it is never embedded in configuration or team manifests. A local AIGW configuration may contain many Accounts; adding a second service never copies or replaces the first service's Token.

Team-manifest import is non-destructive by default. A same-named Account or Runtime Profile is accepted only when its semantic configuration already matches the local object; a mismatch is rejected before any local config projection changes. This prevents a token held in `AIGW_TOKEN/<account>` from being silently redirected to a different endpoint. An operator may explicitly replace a reviewed conflict with `--replace-account <id>` or `--replace-profile <id>`; replacing an Account changes metadata only and never reads, writes, or deletes its Token.

Example:

```toml
[accounts."team-gateway"]
label = "Team Gateway"

[accounts."team-gateway".endpoints]
openai_responses = "https://gateway.example/v1"
anthropic = "https://gateway.example"
```

## Runtime Profile

A Runtime Profile is what users choose day to day. It references one Account and defines a client scope plus the model name to send through that client protocol. One Account may back many Profiles: one Account can back separate GPT and Claude Profiles, while another provider has its own Account and Profile set. Endpoint URLs and provider probes are never Profile fields.

```toml
[profiles."gpt-5.6-terra"]
label = "GPT-5.6"
purpose = "Codex engineering"
account = "team-gateway"
client = "codex"

[profiles."gpt-5.6-terra".models]
codex = "gpt-5.6-terra"
```

`purpose` is optional, human-facing guidance only. It appears in `aigw use`, `aigw profile list`, and `aigw profile show`; it never changes a route, fallback, Account, endpoint, or Token.

Profile and model IDs keep the canonical upstream model name; the client scope
already lives in `client`. AIGW therefore rejects the former GPT client-suffix
alias instead of translating or preserving it as a compatibility path.

`claude-fable-5` is the recommended Claude baseline; Sonnet and Opus are explicit task-specific choices. The compact example team manifest includes `gpt-5.6-terra`, `claude-fable-5`, `claude-sonnet-5`, and `claude-opus-4-8-thinking`; it is not an implicit provider default. Model names are transparent upstream gateway strings; teams can add or remove only the models they have admitted for their own clients. See the [model strategy](model-strategy.md) for the curated capability set and adapter-admission policy.

Use `aigw catalog` to inspect the configured subset and compact count summary of each Account's authenticated OpenAI-compatible model inventory; use `aigw catalog --all` for the full human-readable inventory or `--json` for complete machine output. Then add an explicit Profile with `aigw profile add`. Discovery is read-only: it neither changes a Route nor infers whether an ID supports a particular protocol, embedding, rerank, vision, tools, or reasoning task.

## Endpoint

An Account may provide an Anthropic endpoint, an OpenAI Responses endpoint, or both. Claude consumes the Anthropic endpoint; Codex consumes the OpenAI Responses endpoint. HTTPS is required except for an explicitly configured loopback development Account.

## Provider diagnostics

`aigw check` is generic and probes the current default Profile's Account while separately checking each enabled client's local route and Adapter. It does not silently substitute a client override for the default service or scan unrelated Accounts; use `aigw test --for claude|codex` for an explicit client endpoint check. Exact balance or provider-native Token state is optional: a team manifest may declare an `account_probe`, and the installed AIGW build must explicitly include its Provider Diagnostics implementation. An unknown provider declaration never changes routing or invalidates the Account; it only makes `aigw balance` unavailable with a clear explanation.

## Route

The default route points to a Runtime Profile. Claude and Codex inherit it unless a client-specific override exists:

```bash
aigw use gpt-5.6-terra --for codex
aigw use claude-opus-4-8-thinking --for claude
aigw route reset claude
```

The last command removes the override; it does not duplicate the default route.

A Route is a deterministic local selection before the client request. Without a data-plane Gateway, AIGW never silently retries a request through another service or model.

## Adapter

An Adapter projects a resolved Runtime Profile into one client boundary:

- Claude receives `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `AIGW_ACCOUNT`, `AIGW_PROFILE`, and optional `ANTHROPIC_MODEL` only in the launched process.
- Codex receives an AIGW-marked provider block with Account endpoint and optional model plus credentials through its official `login --with-api-key` command.

Adapters never own provider secrets and never write into one another's directories. Claude shims live in AIGW's dedicated data directory, not in shared `~/.local/bin` or Codex directories. Enabling the adapter adds one bounded, secret-free PATH block to the active user's shell profile so the ordinary `claude` command resolves to the shim; disabling it removes only that owned block.

## External gateway boundary

AIGW is local-first: it is not a gateway, listens on no local port, and is fully usable without a team service. Claude and Codex normally use their Account's HTTPS endpoints directly. A future organization-operated gateway is independently assessed and deployed; AIGW sees only its HTTPS Account endpoint and never manages its process lifecycle, upstream credentials, retries, or fallback policy.

## Installation channel

AIGW records its installation channel at build time:

- `portable`: archive or user-level script install; `aigw update` replaces the current binary atomically.
- `pkg`: macOS package; `aigw update` opens the downloaded installer.
- `deb` / `rpm`: Linux package; `aigw update` invokes the package manager.
- `msi`: Windows Installer package; `aigw update` starts the installer.

This prevents package-manager files from being overwritten by portable
self-update logic.

Portable archives contain local-only `install.sh` and `install.ps1` scripts.
They copy the bundled binary and never retrieve releases or inspect release
credentials. Program distribution has three controlled paths: GitLab is the
formal primary source, GitHub may mirror the exact formal release assets, and a
complete extracted artifact directory supports offline candidate acceptance.
`aigw update` tries GitLab first and may use GitHub only after a source-
availability failure. It never uses a mirror to bypass malformed metadata,
version disagreement, missing artifacts, or checksum failure.

`AIGW_LOCAL_CANDIDATE` names a complete extracted artifact directory. AIGW
accepts it only when it contains exactly one portable archive for the running
platform and that archive validates against the directory's `checksums.txt`. A
source tree or standalone executable is not a candidate. For remote testing,
`AIGW_RELEASE_HOST` and `AIGW_RELEASE_PROJECT` identify GitLab; optional
`AIGW_RELEASE_MIRROR_HOST` and `AIGW_RELEASE_MIRROR_PROJECT` identify GitHub.
The GitLab token fallback requires an explicit HTTPS origin. Tokens are neither
persisted nor placed on a command line; requests have a finite timeout, release
assets remain checksum-verified, the token is removed before a redirect crosses
hosts, HTTPS-to-HTTP redirects are refused, and an older or malformed release
cannot replace the installed binary.

Portable installs retain exactly one immediate predecessor beside the current
binary. `aigw update --rollback` swaps those two local binaries without any
network request or change to configuration, secrets, Accounts, Profiles, Routes
or Adapters. If an older selected program predates this command, download the
current portable package again and run its bundled installer to recover the
current program; it preserves the older program as the one immediate
predecessor. It is intentionally unavailable for native package channels:
native rollback belongs to the operating-system package manager.
