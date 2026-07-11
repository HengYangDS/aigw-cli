# Concepts

## Account

An Account is one upstream provider account boundary: display label, supported protocol endpoints, optional balance probe, and exactly one logical Token. The secret is stored at `AIGW_TOKEN/<account>` in the operating-system credential store; it is never embedded in configuration or team manifests.

Example:

```toml
[accounts.dmx]
label = "DMXAPI"

[accounts.dmx.endpoints]
openai_responses = "https://www.dmxapi.cn/v1"
anthropic = "https://www.dmxapi.cn"
```

## Runtime Profile

A Runtime Profile is what users choose day to day. It references one Account and may define a client scope and model name.

```toml
[profiles."gpt-5.6-sol-cdx"]
label = "GPT-5.6"
account = "dmx"
client = "codex"

[profiles."gpt-5.6-sol-cdx".models]
codex = "gpt-5.6-sol-cdx"
```

Built-in examples include `gpt-5.6-sol-cdx`, `gpt-5.5`, `gpt-5.5-ssvip`, `claude-sonnet-5`, `claude-opus-4-8-thinking`, and `claude-fable-5`. Model names are transparent upstream gateway strings; teams can add or remove them in their manifest.

## Endpoint

An Account may provide an Anthropic endpoint, an OpenAI Responses endpoint, or both. Claude consumes the Anthropic endpoint; Codex consumes the OpenAI Responses endpoint. HTTP is accepted only for loopback compatibility tools such as a separately managed local proxy.

## Route

The default route points to a Runtime Profile. Claude and Codex inherit it unless a client-specific override exists:

```bash
aigw use gpt-5.6-sol-cdx --for codex
aigw use claude-opus-4-8-thinking --for claude
aigw route reset claude
```

The last command removes the override; it does not duplicate the default route.

## Adapter

An Adapter projects a resolved Runtime Profile into one client boundary:

- Claude receives `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `AIGW_ACCOUNT`, `AIGW_PROFILE`, and optional `ANTHROPIC_MODEL` only in the launched process.
- Codex receives an AIGW-marked provider block with Account endpoint and optional model plus credentials through its official `login --with-api-key` command.

Adapters never own provider secrets and never write into one another's directories. Claude shims live in AIGW's dedicated data directory, not in shared `~/.local/bin` or Codex directories. Enabling the adapter adds one bounded, secret-free PATH block to the active user's shell profile so the ordinary `claude` command resolves to the shim; disabling it removes only that owned block.

## Optional data-plane proxy

AIGW is not a proxy and listens on no local port. Claude and Codex normally use their Account's HTTPS endpoints directly. A separately operated proxy is justified only for protocol translation, required centralized audit, or a mandated network egress path. When one is used, its endpoint is an Account URL and its port belongs to that proxy project; AIGW never manages its process lifecycle. Do not select `8888` merely as a convention—choose and reserve a port in the proxy project's deployment contract after checking local and organizational conflicts.

## Installation channel

AIGW records its installation channel at build time:

- `portable`: archive or user-level script install; `aigw update` replaces the current binary atomically.
- `pkg`: macOS package; `aigw update` opens the downloaded installer.
- `deb` / `rpm`: Linux package; `aigw update` invokes the package manager.
- `msi`: Windows Installer package; `aigw update` starts the installer.

This prevents package-manager files from being overwritten by portable self-update logic.
