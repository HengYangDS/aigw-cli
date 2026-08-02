# Security model

## Secret storage

| Platform | Default backend |
|---|---|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux | Secret Service over D-Bus |

The logical service is always `AIGW_TOKEN`; the account is the Account ID, not the Runtime Profile ID. Linux without a usable Secret Service fails explicitly. CI may set `AIGW_SECRET_BACKEND=env` to select the read-only environment backend and expose `AIGW_TOKEN_<NORMALIZED_ACCOUNT>` to the process. When that credential is pre-provisioned, non-interactive `aigw setup` reuses it to validate and write only secret-free metadata; it neither prompts for nor persists a duplicate Token. An explicit `--token-stdin` remains a request to write a new credential and is therefore rejected by the read-only environment backend.

Importing a token-free configuration manifest cannot silently redirect an existing Account Token: an Account or Profile collision must be semantically identical or the import fails before mutation. Reviewed replacement requires `aigw config import ... --replace-account <id>` and/or `--replace-profile <id>`; Account replacement updates only public metadata and retains the existing system-secret slot unchanged.

Account renaming (`aigw account rename`) preserves this boundary through a two-phase credential migration. Phase 1 copies missing target Token and optional `AIGW_ACCOUNT/<account>` account-probe credentials and reads them back; equal target values are resumable, while differing values fail closed. The read-only `env` backend requires equal target variables to be externally pre-provisioned. After success, the current TOML has no old Account key; only the single `.bak` configuration preimage and the old credential slots retain the old identity.

Finalization requires both Account IDs, semantic agreement with the complete admitted-client verified checkpoint, and an available target Token. It converges the single `.bak` under a three-file exact-preimage check before deleting old slots. Each rotation confirmation flag is required only when its corresponding old and new slots differ, and differing probe credentials trigger a live target-provider probe during apply. Partial deletion is retryable; old `env` variables must be unset externally before the incomplete, non-zero finalization is retried.

Configuration, the Token secret store, and the account-probe secret store do not form an ACID transaction. The three-file exact-preimage check is best-effort protection and is not a cross-process CAS guarantee; a detected competing change fails closed rather than proving global atomicity.

Optional provider platform credentials for exact balance diagnostics are stored separately from API Tokens. Exact diagnostics are explicitly selected provider integrations, not a prerequisite for generic Account health checks or a hidden provider default.

## Non-persistence rules

Tokens must not enter:

- AIGW TOML or configuration manifests
- command-line arguments or shell history
- Claude settings or Codex provider configuration
- JSON output, diagnostics, logs, documentation, backups, or SBOMs

Any response text that can reach a diagnostic or provider error crosses a shared
redaction boundary first: known credentials are removed in plain and URL-escaped
forms, bearer credentials are removed even when their value is unknown, and
credential-shaped JSON/query fields are redacted without discarding unrelated
diagnostic context.

The credential-literal gate rejects `sk-` and bearer-token patterns in
tracked non-test source. Test fixtures use explicit `aigw-test-*` sentinels
rather than API-key-shaped literals, and the credential-fixture gate rejects
such patterns in test source. These controls prevent source, history reviews,
and evidence retention from mistaking fixture-only data for a live secret.

Use hidden terminal input or pipe exactly one line with `--token-stdin`. AIGW rejects endpoint URLs containing userinfo or credential-like query parameters. Remote endpoints require HTTPS; HTTP is loopback-only.

## Client boundaries

The AIGW-owned Claude launcher invokes the AIGW binary's hidden adapter
boundary. AIGW refuses to overwrite or remove a foreign `claude` launcher. On
macOS/Linux, the launcher lives beneath AIGW's data directory instead of shared
`~/.local/bin`; the Adapter writes one bounded, secret-free PATH block to the
active user's shell profile. On Windows, it is a `.cmd` file in the AIGW-owned
data directory and names the configured absolute `aigw.exe`. This avoids
package-manager-owned directories and shared-bin cleanup races. The PATH block
and launcher are removed together when the Adapter is disabled. `aigw doctor`
rejects a launcher whose target is missing, malformed, non-executable on Unix,
or under a temporary directory, and directs the user to `aigw repair`.

The portable Unix installer also starts from a fixed system-tool PATH. If it was
invoked with a zsh PATH that lacks the system directories needed before
`.zshrc` runs, it writes a separate AIGW-owned, secret-free conditional
`.zshenv` bootstrap; normal shells do not receive that file, and portable
uninstall removes only its delimited block. Portable uninstall uses the same
fixed system-tool PATH, so removal remains possible from a restricted shell.

`aigw doctor` rejects globally inherited client-token variables and reports names only, never values. Remove them from login, shell, IDE, and launch-agent environments. `AIGW_TOKEN_<ACCOUNT>` remains the explicit CI-only secret backend; it is not a global client credential. The Claude launcher injects the Account Token as `ANTHROPIC_AUTH_TOKEN` only into the Claude process it launches.

Codex ownership is target-specific. AIGW may manage top-level `model` and
`model_provider` selections plus a delimited `[model_providers.aigw]` block
only for an admitted Codex Home target. The default home is shared by Codex CLI
and Codex Desktop and is discovered at `~/.codex/config.toml`; every additional
home must be configured explicitly. Desktop-only GUI settings, IDE settings,
client sessions, conversation JSONL, SQLite, selected models, transcripts, and
application lifecycle are outside AIGW.

AIGW snapshots each owned configuration and sidecar as exact bytes, existence,
digest, and POSIX mode. Before a write it verifies the captured preimage;
compensating rollback restores only its own unchanged postimages. This guards
against ordinary concurrent edits but is not a cross-process CAS guarantee.

`aigw sync` reconciles only configured Codex Home targets. It never starts,
stops, restarts, or reloads a Claude/Codex client and never rebinds credentials
during a dry-run. Credential binding is limited to first Codex adapter enable,
an Account-changing Codex route, a Token rotation, or the explicit
`aigw adapter auth codex` command. Each native `codex login --with-api-key`
invocation has a 20-second bound and receives the Token only through stdin.

`aigw repair --dry-run` renders proposed Codex Home adoption and restore actions
without writing AIGW configuration, Codex files, sidecars, launchers, or
credentials; it neither runs native login nor acquires the configuration
mutation lock.

Endpoint checks are bounded HTTP requests. AIGW does not use an unbounded `codex exec` process as a health check and does not install a watchdog or any lifecycle automation for desktop clients.

`aigw test` is a bounded connectivity check. A `404` to an authenticated GET of the Claude base URL means the service is reachable but does not expose a base GET probe; it is not classified as an endpoint outage. `aigw verify` is explicitly opt-in and consumes a minimal real model request; it requires the exact `AIGW_OK` sentinel rather than accepting a successful HTTP status or process exit alone. Claude verification explicitly pins the resolved model and invokes Claude in safe mode with hooks, MCP, skills, plugins, custom commands, and session persistence disabled. Its captured child invocation has a bounded pipe-drain wait; on Windows, its direct child is placed in a kill-on-close Job Object so descendants created by that invocation cannot outlive the verification boundary. `aigw verify --for all` first verifies local Claude-launcher and Codex-projection readiness, then writes a secret-free verified checkpoint only after both clients pass. `aigw rollback` restores that checkpoint (or `--last-change` restores only the immediate config backup) through the normal projection transaction; it never controls the lifecycle of a desktop client.

Portable `install.sh --help` / `uninstall.sh --help` and PowerShell `-Help`
exit before any mutation. Every portable upgrade path--archive installer and
`aigw update` alike--retains one immediately preceding binary at
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

Portable uninstall removes the binary, its own user's AIGW-owned Claude launcher, and the bounded AIGW Claude PATH block. Native package uninstall deliberately manages only package-owned files; it never searches or deletes another user's launcher or shell configuration. Disable the Claude adapter as the target user before native uninstall when launcher removal is desired. All uninstall paths preserve configuration, system-keyring entries, account diagnostics credentials, and user-owned client configuration. Remove Account secrets from the operating-system credential store only when offboarding policy requires it; removing a Runtime Profile does not delete its Account Token.
