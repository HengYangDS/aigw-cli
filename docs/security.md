# Security model

## Secret storage

| Platform | Default backend |
|---|---|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux | Secret Service over D-Bus |

The logical service is always `AIGW_TOKEN`; the account is the Account ID, not the Runtime Profile ID. Linux without a usable Secret Service fails explicitly. CI may set `AIGW_SECRET_BACKEND=env` to select the read-only environment backend and expose `AIGW_TOKEN_<NORMALIZED_ACCOUNT>` to the process. When that credential is pre-provisioned, non-interactive `aigw setup` reuses it to validate and write only secret-free metadata; it neither prompts for nor persists a duplicate Token. An explicit `--token-stdin` remains a request to write a new credential and is therefore rejected by the read-only environment backend.

Importing a token-free team manifest cannot silently redirect an existing Account Token: an Account or Profile collision must be semantically identical or the import fails before mutation. Reviewed replacement requires `aigw config import ... --replace-account <id>` and/or `--replace-profile <id>`; Account replacement updates only public metadata and retains the existing system-secret slot unchanged.

Optional provider platform credentials for exact balance diagnostics are stored separately from API Tokens. Exact diagnostics are explicitly selected provider integrations, not a prerequisite for generic Account health checks or a hidden provider default.

## Non-persistence rules

Tokens must not enter:

- AIGW TOML or team manifests
- command-line arguments or shell history
- Claude settings or Codex provider configuration
- JSON output, diagnostics, logs, documentation, backups, or SBOMs

Any response text that can reach a diagnostic or provider error crosses a shared
redaction boundary first: known credentials are removed in plain and URL-escaped
forms, bearer credentials are removed even when their value is unknown, and
credential-shaped JSON/query fields are redacted without discarding unrelated
diagnostic context.

Use hidden terminal input or pipe exactly one line with `--token-stdin`. AIGW rejects endpoint URLs containing userinfo or credential-like query parameters. Remote endpoints require HTTPS; HTTP is loopback-only.

## Client boundaries

The Claude shim is marked as AIGW-owned and invokes the AIGW binary's hidden adapter boundary. AIGW refuses to overwrite or remove a foreign `claude` launcher. On macOS/Linux it is created beneath AIGW's data directory rather than shared `~/.local/bin`; the Adapter writes one bounded, secret-free PATH block to the target user's shell profile. On Windows it is a `.cmd` file in the AIGW-owned data directory and points to the configured absolute `aigw.exe`, rather than assuming the two files share a directory. This avoids both package-manager-owned system directories and shared-bin cleanup races. The block and launcher are both AIGW-owned and removed together when the Adapter is disabled. `aigw doctor` rejects a shim whose target is missing, malformed, or (on Unix) non-executable or under a disposable temporary directory, and directs the user to `aigw repair` before it becomes a silent Claude outage.

The portable Unix installer also starts from a fixed system-tool PATH. If it was
invoked with a zsh PATH that lacks the system directories needed before
`.zshrc` runs, it writes a separate AIGW-owned, secret-free conditional
`.zshenv` bootstrap; normal shells do not receive that file, and portable
uninstall removes only its delimited block. Portable uninstall uses the same
fixed system-tool PATH, so removal remains possible from a restricted shell.

`aigw doctor` rejects globally inherited client-token variables and reports names only, never values. Remove them from login, shell, IDE, and launch-agent environments. `AIGW_TOKEN_<ACCOUNT>` remains the explicit CI-only secret backend; it is not a global client credential. The Claude shim injects the Account Token as `ANTHROPIC_AUTH_TOKEN` only into the Claude process it launches.

Codex changes consist only of AIGW-owned top-level `model` and `model_provider` selections plus a delimited `[model_providers.aigw]` block. AIGW keeps a per-target state snapshot, validates all three owned surfaces against the resolved Profile in `aigw doctor`, and preserves user edits elsewhere in `config.toml`. If a formatter removes only AIGW ownership comments, AIGW accepts recovery only when the state hash and every owned model/provider value exactly match the selected Profile; any semantic difference remains a conflict.

`aigw sync` only reconciles those owned config surfaces. It never starts, stops, restarts, or reloads a Claude/Codex client and never rebinds credentials. Credential binding is limited to first Codex adapter enable, an Account-changing Codex route, a Token rotation, or the explicit `aigw adapter auth codex` command; each native `codex login --with-api-key` invocation has a 20-second bound and receives the Token only through stdin.

Endpoint checks are bounded HTTP requests. AIGW does not use an unbounded `codex exec` process as a health check and does not install a watchdog or any lifecycle automation for desktop clients.

`aigw test` is a bounded connectivity check. A `404` to an authenticated GET of the Claude base URL means the service is reachable but does not expose a base GET probe; it is not classified as an endpoint outage. `aigw verify` is explicitly opt-in and consumes a minimal real model request; it requires the exact `AIGW_OK` sentinel rather than accepting a successful HTTP status or process exit alone. Claude verification explicitly pins the resolved model and invokes Claude in safe mode with hooks, MCP, skills, plugins, custom commands, and session persistence disabled. Its captured child invocation has a bounded pipe-drain wait; on Windows, its direct child is placed in a kill-on-close Job Object so descendants created by that invocation cannot outlive the verification boundary. `aigw verify --for all` first verifies local Claude-shim and Codex-projection readiness, then writes a secret-free verified checkpoint only after both clients pass. `aigw rollback` restores that checkpoint (or `--last-change` restores only the immediate config backup) through the normal projection transaction; it never controls the lifecycle of a desktop client.

Portable `install.sh --help` / `uninstall.sh --help` and PowerShell `-Help`
exit before any mutation. Every portable upgrade path—archive installer and
`aigw update` alike—retains one immediately preceding binary at
`<install-dir>/.aigw.previous` (or `.aigw.previous.exe` on Windows). The next
portable upgrade replaces that one rollback copy; portable uninstall removes
only that AIGW-owned rollback binary together with the installed executable.
`aigw update --rollback` uses that same single portable rollback copy, swaps
it with the active binary locally, and never contacts a release server or
touches AIGW configuration, secrets or client projections. If a selected
legacy binary lacks the command, download the current portable archive again
and run its installer; that installer copies only its bundled binary, retains
one predecessor, and neither reads tokens nor retrieves a release.

## Update boundary

Portable installs may replace their own binary after checksum verification. Native package installs use their native installer path: `.pkg`, `.deb`, `.rpm`, or `.msi`. This keeps package-manager ownership intact.

## Uninstall

Portable uninstall removes the binary, its own user's AIGW-owned Claude launcher, and the bounded AIGW Claude PATH block. Native package uninstall deliberately manages only package-owned files; it never searches or deletes another user's shim or shell configuration. Disable the Claude adapter as the target user before native uninstall when shim removal is desired. All uninstall paths preserve configuration, system-keyring entries, account diagnostics credentials, and user-owned client configuration. Remove Account secrets from the operating-system credential store only when offboarding policy requires it; removing a Runtime Profile does not delete its Account Token.
