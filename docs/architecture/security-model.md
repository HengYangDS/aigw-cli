---
subject: aigw:security-model
role: explanation
state: canonical
relations:
  canonical_for: credential, process, and trust boundaries
---

# Security Model

AIGW keeps credentials local, mutations bounded, and client ownership explicit.

## Credential storage

- **Provider Tokens** use one selected local backend. They never appear in
  repository files or public configuration.
- **Balance credentials** use a separate typed slot in that same backend.
  Their balance Token and user ID are both required; neither substitutes for a
  provider Token. They never appear in repository files or public configuration.
- **Forge credentials** belong to the protected CI or operator process, not
  AIGW's provider Token store. They are never tracked in Git.

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

- **`keyring`** uses macOS Keychain or Linux Secret Service. Every value or metadata operation runs through the invoking AIGW
  executable, the installed program or a versioned reader, as a bounded
  credential subprocess with a five-second deadline and
  bounded process cleanup. The subprocess receives only the platform identity environment it needs;
  writes carry the logical Token only through standard input. Metadata queries
  never request credential bytes. Explicit selection fails closed if the native
  service is unavailable; AIGW neither retries through another backend nor
  changes access policy. Linux reads the collection and item lock metadata before
  value access or mutation; locked items fail without requesting an unlock or
  invoking a native prompt. Pending prompts returned by native creation or
  deletion are dismissed, not displayed. The subprocess deadline bounds cleanup;
  it is not a substitute for refusing native authorization UI.
- **`file`** uses an owner-only directory and regular file per provider. It
  uses bounded paths and same-directory replacement; the selected backend
  supports credential mutation.
- **`env`** consumes credentials supplied to the invoking process on every
  supported OS. It is read-only: setup may persist public configuration, but
  storing, rotating or deleting an environment Token fails before mutation.

An item can be visible to metadata queries while its access policy denies value
access. Existing Token bytes and their storage location are not evidence of
authorization. Native acceptance therefore invokes the exact released AIGW
credential command against a retained item; a source fixture or another helper's
successful read is insufficient.

Supply environment credentials to the process that needs them. A Token set in
one terminal is not automatically inherited by a separately launched GUI client.
The projected helper runs in the client's environment. See
[automated rollout](../guides/team-rollout.md#automated-rollout) for API-Token
variable names, reversible provider-ID encoding, and process scope.

Unattended work requires an already-proven noninteractive credential boundary;
it must not assume metadata access is sufficient.

### AIGW-owned credential entrypoint

For a client that uses a provider's Token, AIGW projects the existing
`aigw credential` protocol through a user-private, SHA-256-addressed copy of
the same executable under its platform data directory. A successor gets a new
path rather than overwriting old bytes. It uses the same selected backend and
Token slots; it is not another store or helper. The paired SHA-256 receipt
detects byte drift, not release provenance or native-store authorization.
Creation participates in projection compensation.

Ordinary sync and client withdrawal retain an existing copy:
absence from current configuration does not prove cached or rollback callers
have stopped using it. Explicit portable uninstall withdraws projections and
removes the selected installation executable and its rollback copy, even when
invoked from a different AIGW binary. It leaves versioned readers and receipts
unchanged: a content-addressed reader may also serve another installation or
cached caller. It does not reauthorize a withdrawn client; a cached
command using the withdrawn configuration must fail closed. Exact reader
cleanup requires proof that no configured, cached, explicit, rollback, or
other-installation caller remains; neither uninstall nor a prefix or age match
supplies that proof. AIGW therefore never removes a versioned reader itself.

An already-running client that cached the old package-manager path is not
migrated by rewriting settings: after an upgrade from 0.3.5 or earlier it fails
until its client is set up again. Later versioned paths remain retained for
their original callers.

A macOS Keychain item that a signed AIGW release creates trusts the signing
identifier and team, not one executable's bytes. Every later signed release
reads it without a prompt, so an upgrade, a program rollback and a Homebrew
downgrade move no item. AIGW uses an item only through that identity: it runs
no other program to read, replace or delete one, and no command changes an
item's owner. An item that 0.3.5 or earlier stored trusts only
`/usr/bin/security`. macOS refuses AIGW the read of it, and AIGW does not
replace an item that it cannot read: macOS would accept the new value and
still refuse the read (measured on 2026-11-15). Such an item is no Token that
AIGW can use. A command that needs the credential for its own purpose ends
with the refusal, which says that another program stored the item and names
the fix: have the credential at hand, delete the item with that tool and store
the credential again. `aigw status` and `aigw check --depth local` report the
item with that fix, and `aigw provider show`, which reads no Token, reports it
as stored. A command that writes the files of several clients leaves the client
of that provider waiting, as it does for a missing Token, and completes for the
others: it decides from an observation that returns no value, before it
installs a credential reader, so no client file is written to name a reader
that would return no Token. An ad-hoc or development build has another identity
and meets the same refusal. Deleting the item for such a build takes the Token
from the installed release, so a development build uses a home of its own.
macOS gives a locked keychain's refusal the same status, so the credential
worker also reports whether the keychain that holds the item was unlocked,
before the operation and after its refusal, and only a refusal with a keychain
unlocked at both times counts as another program's item. macOS does let AIGW
delete such an item, so `aigw provider remove` removes it as it removes AIGW's
own. A migration that a release from 0.4.1 to 0.7.2 began and did not finish
can have left an item named `migration@<slot>` under the service `AIGW_TOKEN`.
AIGW neither reads, lists nor removes it;
`security delete-generic-password -s AIGW_TOKEN -a migration@<slot>` removes
it. The
[credential decision](../decisions/dr-0011-single-portable-token-backend.md)
records why the earlier one-time migration was retired.

The [credential-continuity design](../../openspec/changes/archive/2026-10-16-terminal-product-convergence/design.md#4-decide-credential-command-continuity-before-cutover)
defines native qualification. The [frozen-source and delivery tasks](../../openspec/changes/archive/2026-10-16-terminal-product-convergence/tasks.md#9-frozen-source-and-pre-archive-acceptance)
must also pass before this path is a released guarantee.

### External credential executable

Each local client may set `credential_command` to one absolute executable
path. The selected executable must accept
`credential <client> <fingerprint>`, validate the requested projection,
and emit only its Token on successful stdout. The fingerprint identifies a
client, provider and endpoint; it is not caller authorization. The operator owns the executable's
installation, trust, permissions and credential source. AIGW does not discover
an alternative reader, change native access control or install the helper.

This explicit client projection does not change `aigw credential` or backend
selection. Without it, the AIGW-owned credential entrypoint remains the reader.
Sync, check and dry-run do not execute the external helper; they cannot
prove that it can retrieve a Token. With an external helper, check and status
report only local projection readiness and direct the operator to live
verification instead of requiring a duplicate Token in AIGW's store.
Direct `aigw check --depth endpoint` does not invoke an external helper: use
`aigw check <client> --depth client`, or supply a one-time Token with
`--model <provider/model> --token-stdin`.

Explicit live verification uses the selected
native client's synchronized settings. Claude verification does not first read
AIGW's native store when this policy is present. A failure hides raw client
diagnostics because AIGW does not know the external Token to redact it;
cancellation and the bounded verification deadline remain observable.

Disabling the client retains its explicit credential policy without
automatically enabling it on a later sync. Full withdrawal removes that local
policy, but does not remove the external executable or its credential store.
Replacing the policy requires ordinary guarded projection reconciliation.
No release older than 0.7.0 reads `aigw.toml`, so program rollback cannot
return to one; the updater's compatibility check refuses a retained program
that cannot read the current configuration, and changes nothing.

## Configuration boundary

Team file validation uses public metadata, not Token values. A client that
uses a provider's Token requires credential availability before activation. Client-native
Codex authentication needs no AIGW Token and remains client-owned. Public
configuration never carries credentials.

A team file never changes an existing provider's endpoints or Token: a
differing endpoint stays local and is reported, because a file must not
redirect a Token. Replacing provider metadata preserves its Token slot, and
`aigw provider edit` changes an endpoint deliberately. An endpoint change
redirects future requests and therefore requires review; the old projection
fingerprint cannot retrieve a Token for the new endpoint.

## Client boundary

| Client                    | AIGW-owned projection                                             | Preserved client state                                                    |
| ------------------------- | ----------------------------------------------------------------- | ------------------------------------------------------------------------- |
| Codex                     | Recorded provider, selections, scheduler, catalog and sidecar     | Native credentials, conversations, models chosen in the App and GUI state |
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
- An explicit `aigw check` may consume quota because it probes selected-model
  inference by default; `--depth endpoint` avoids that inference request.
- An explicit `aigw check --depth client` may make multiple quota-consuming
  real-client requests. Without named clients it writes a configuration
  checkpoint only after no client failed.

A 401 or 403 is classified immediately after one bounded request; repeated
credential probes cannot authorize access. Mixed non-authentication results or
cancellation remain instability rather than a credential diagnosis. This
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

Errors name the problem and one exact fix, with bounded evidence where it adds a fact.

## Update boundary

Local Git supplies the single signed product tag. Each selected Forge receives
that exact tag object and independently supplies its Release record and assets.
AIGW never combines a tag, checksum manifest, or artifact across peers.
Authentication, object identity, metadata, checksum, archive-layout, downgrade,
and redirect failures are terminal.

## Uninstall

Uninstall first withdraws AIGW-owned client projections, including marked
configuration blocks, sidecars, generated catalogs, and credential helpers.
It then removes the selected program and its rollback copy. Providers, models,
client selections, Tokens, explicit configuration backup, client conversations, and
neighboring user-authored settings remain intact.
