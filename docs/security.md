# Security model

## Secret storage

| Platform | Default backend |
|---|---|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux | Secret Service over D-Bus |

The logical service is always `AIGW_TOKEN`; the account is the Account ID, not the Runtime Profile ID. Linux without a usable Secret Service fails explicitly. CI may set `AIGW_SECRET_BACKEND=env` to select the read-only environment backend and expose `AIGW_TOKEN_<NORMALIZED_ACCOUNT>` to the process.

Optional provider platform credentials for exact balance diagnostics are stored separately from API Tokens.

## Non-persistence rules

Tokens must not enter:

- AIGW TOML or team manifests
- command-line arguments or shell history
- Claude settings or Codex provider configuration
- JSON output, diagnostics, logs, documentation, backups, or SBOMs

Use hidden terminal input or pipe exactly one line with `--token-stdin`. AIGW rejects endpoint URLs containing userinfo or credential-like query parameters. Remote endpoints require HTTPS; HTTP is loopback-only.

## Client boundaries

The Claude shim is marked as AIGW-owned and invokes the AIGW binary's hidden adapter boundary. AIGW refuses to overwrite or remove a foreign `claude` launcher. The shim is created in the user-level AIGW shim directory, not in `~/.codex` and not in package-manager owned system directories.

Codex changes consist only of AIGW-owned top-level `model` and `model_provider` selections plus a delimited `[model_providers.aigw]` block. AIGW keeps a per-target state snapshot, validates all three owned surfaces against the resolved Profile in `aigw doctor`, and preserves user edits elsewhere in `config.toml`.

`aigw sync` only reconciles those owned config surfaces. It never starts, stops, restarts, or reloads a Claude/Codex client and never rebinds credentials. Credential binding is limited to first Codex adapter enable, an Account-changing Codex route, a Token rotation, or the explicit `aigw adapter auth codex` command; each native `codex login --with-api-key` invocation has a 20-second bound and receives the Token only through stdin.

Endpoint checks are bounded HTTP requests. AIGW does not use an unbounded `codex exec` process as a health check and does not install a watchdog or any lifecycle automation for desktop clients.

`aigw test` is a bounded connectivity check. `aigw verify` is explicitly opt-in and consumes a minimal real model request; it requires the exact `AIGW_OK` sentinel rather than accepting a successful HTTP status or process exit alone. `aigw verify --for all` first verifies local Claude-shim and Codex-projection readiness, then writes a secret-free verified checkpoint only after both clients pass. `aigw rollback` restores that checkpoint (or `--last-change` restores only the immediate config backup) through the normal projection transaction; it never controls the lifecycle of a desktop client.

## Update boundary

Portable installs may replace their own binary after checksum verification. Native package installs use their native installer path: `.pkg`, `.deb`, `.rpm`, or `.msi`. This keeps package-manager ownership intact.

## Uninstall

Portable uninstall removes the binary and its own user's AIGW-owned Claude launcher. Native package uninstall deliberately manages only package-owned files; it never searches or deletes another user's shim. Disable the Claude adapter as the target user before native uninstall when shim removal is desired. All uninstall paths preserve configuration, system-keyring entries, account diagnostics credentials, and user-owned client configuration. Remove Account secrets from the operating-system credential store only when offboarding policy requires it; removing a Runtime Profile does not delete its Account Token.
