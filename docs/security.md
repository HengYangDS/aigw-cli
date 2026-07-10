# Security model

## Secret storage

| Platform | Default backend |
|---|---|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux | Secret Service over D-Bus |

The logical service is always `AIGW_TOKEN`; the account is the Profile ID. Linux without a usable Secret Service fails explicitly. CI may set `AIGW_SECRET_BACKEND=env` to select the read-only environment backend and expose `AIGW_TOKEN_<NORMALIZED_PROFILE>` to the process.

## Non-persistence rules

Tokens must not enter:

- AIGW TOML or team manifests
- command-line arguments or shell history
- Claude settings or Codex provider configuration
- JSON output, diagnostics, logs, documentation, backups, or SBOMs

Use hidden terminal input or pipe exactly one line with `--token-stdin`. AIGW rejects endpoint URLs containing userinfo or credential-like query parameters. Remote endpoints require HTTPS; HTTP is loopback-only.

## Client boundaries

The Claude shim is marked as AIGW-owned and invokes the AIGW binary's hidden adapter boundary. AIGW refuses to overwrite or remove a foreign `claude` launcher.

Codex changes consist only of an AIGW-owned `model_provider` selection and a delimited `[model_providers.aigw]` block. AIGW validates those two owned surfaces before sync or rollback. User edits elsewhere in `config.toml` are preserved.

## Uninstall

Uninstall removes the binary and AIGW-owned Claude launcher. It deliberately preserves configuration, system-keyring entries, and user-owned client configuration. Remove a Profile secret explicitly with `aigw profile remove <name>` before uninstall if policy requires it.
