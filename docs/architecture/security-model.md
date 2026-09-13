# Security Model

AIGW keeps credentials local, mutations bounded, and client ownership explicit.

## Credential storage

- **Account Tokens** use one selected local backend. They never appear in
  repository files or public configuration.
- **Diagnostic credentials** use a separate typed slot in that same backend.
  Their system Token and user ID are both required; neither substitutes for an
  Account Token. They never appear in repository files or public configuration.
- **Forge credentials** belong to the protected CI or operator process, not
  AIGW's Account store. They are never tracked in Git.

Automatic selection first honors an existing backend choice. Without one, it
attempts native-service metadata access and selects that backend if the probe
succeeds; otherwise it selects one AIGW-owned fallback store. The probe does
not read Tokens or prove future read/write permission. AIGW never searches or
writes both stores.

Read-only commands do not persist a new choice. The first credential mutation
records its selected backend before changing Token state; a failed mutation
compensates only the choice it created. Once persisted, an unavailable backend
fails closed rather than silently choosing another store. To require a specific
mechanism, set `AIGW_SECRET_BACKEND` to
`keyring`, `file`, or `env` before running AIGW:

- **`keyring`** uses macOS Keychain, Linux Secret Service or Windows Credential
  Manager. Reads and writes require that native service's access permission.
  Explicit selection fails closed if the service is unavailable; AIGW does not
  silently switch stores. Metadata observation does not authorize secret reads
  or guarantee that a native service will never request interaction.
- **`file`** uses an owner-only directory and regular file per Account on macOS
  and Linux. Windows encrypts each Token with current-user DPAPI before writing
  it beneath the AIGW data directory. Both use bounded paths and same-directory
  replacement; the selected backend supports credential mutation.
- **`env`** consumes credentials supplied to the invoking process on every
  supported OS. It is read-only: setup may persist public configuration, but
  storing, rotating or deleting an environment Token fails before mutation.

Supply environment credentials to the process that needs them. A Token set in
one terminal is not automatically inherited by a separately launched GUI client.
The projected helper runs in the client's environment. See the
[environment-variable reference](../../README.md#environment-variables) for
API-Token and diagnostic variable names, reversible Account-ID encoding and
process scope. Unattended work requires an already-proven noninteractive
credential boundary; it must not assume metadata access is sufficient.

## Configuration boundary

Manifest validation uses public metadata, not Token values. Account-Token
routes require credential availability before client activation. Client-native
Codex authentication needs no AIGW Token and remains client-owned. Public
configuration never carries credentials.

A manifest collision must be semantically identical or explicitly replaced.
Replacing Account metadata preserves its Token slot. An explicit endpoint
change redirects future requests and therefore requires review; the old
projection fingerprint cannot retrieve a Token for the new endpoint.

## Client boundary

| Client                    | AIGW-owned projection                                             | Preserved client state                                                    |
| ------------------------- | ----------------------------------------------------------------- | ------------------------------------------------------------------------- |
| Codex                     | Recorded provider, selections, scheduler, catalogue and sidecar   | Native credentials, conversations, models chosen in the App and GUI state |
| Claude Code               | Endpoint, guarded model preference, credential helper and sidecar | Native credentials, sessions, shell profiles and unrelated settings       |
| Missing or foreign client | None                                                              | Existing files, directories and runtime state                             |

The [client projection contract](authority-and-projection-boundary.md#client-boundaries)
defines the exact source ranges and guarded writes. Codex JSONL, SQLite and
conversation metadata remain client-owned; decorative comments grant no
additional write authority.

## Transaction boundary

Every multi-file mutation:

1. validates the complete desired state;
2. captures exact preimages;
3. prepares all writes before the first commit;
4. checks the expected preimage before each write;
5. compensates only unchanged owned postimages, continuing independent recovery
   after a conflict and preserving all failure causes.

These checks preserve detected newer writes. They are best-effort filesystem
guards, not cross-process CAS against editors that ignore AIGW's mutation lock.
The [transaction model](authority-and-projection-boundary.md#configuration-transaction)
owns sequencing and compensation details.

## Network boundary

- Endpoint verification is bounded and explicit.
- Redirects never forward credentials across origins.
- HTTPS-to-HTTP redirect is rejected.
- Tokens are not placed on command lines or persisted in logs.
- A loopback endpoint is not proof of listener health or ownership.
- `aigw verify` may consume quota only when the operator requests it.

An initial 401 is transient only when three bounded observations recover, and a
Token is classified as persistently invalid only after three further 401
responses. Mixed results or cancellation remain retryable instability. This
single-command observation covers one configured endpoint and in-memory Token;
it does not prove direct-upstream health, account or billing state, or a later
request.

## Output boundary

User-facing output excludes Token values and inherited client-token environment
values. The credential-helper protocol deliberately writes only its requested
Token to the invoking client's stdout; it is not a diagnostic or export surface.
Diagnostics expose bounded, redacted response details rather than unbounded
raw bodies. Explicit inspection and export commands may show configuration
paths, endpoints or public configuration; those are not secret-free telemetry
for public redistribution. Review such output before sharing it.

Errors name the problem, bounded evidence, impact, and one recommended action.

## Update boundary

Local Git supplies the single signed product tag. Each selected Forge receives
that exact tag object and independently supplies its Release record and assets.
AIGW never combines a tag, checksum manifest, or artifact across peers.
Authentication, object identity, metadata, checksum, archive-layout, downgrade,
and redirect failures are terminal.

## Uninstall

Uninstall first withdraws AIGW-owned client projections, including marked
configuration blocks, sidecars, generated catalogues, and credential helpers.
It then removes the selected program and its rollback copy. Accounts, Profiles,
Routes, Tokens, explicit configuration backup, client conversations, and
neighboring user-authored settings remain intact.
