# Team Rollout

## Maintainer workflow

Publish a reviewed, token-free account and profile manifest from the adopting
team's own controlled configuration repository. Use the deliberately fictitious
[`manifests/example.toml`](../../manifests/example.toml) only as the
provider-neutral product-format example. A manifest may contain account
metadata, endpoints, profiles, models, purposes, a recommended default, and
optional recommended client routes. It must not contain tokens, adapter state,
or machine paths.

Configuration manifests use the single v3 schema. Older clients must be updated
before rollout rather than partially applying an unsupported schema.

```bash
aigw config export > manifest.toml
```

Exported client routes are recommendations. Import and setup fill only a
member's missing route selections; they do not replace an existing personal
Claude or Codex route.

Import is idempotent only for semantically identical names. A conflict stops
before mutation. A reviewed operator can replace public metadata explicitly:

```bash
aigw config import manifest.toml --replace-account <account>
aigw config import manifest.toml --replace-profile <profile>
```

Those switches never read, copy, delete, or replace a member's system token.

## Member workflow

### New machine

Review each Account's endpoint and optional
`codex_responses_storage` requirement. Profiles, models, and recommended routes
are maintained team metadata; Tokens never belong in this file. The team's
deployment manifest may use a provider endpoint directly or an independently
managed compatibility endpoint. Preserve each reviewed storage requirement
when changing an endpoint because it governs the provider's Responses item
policy.

```bash
aigw setup --from manifest.toml
aigw check
```

A separately managed Responses compatibility proxy is opt-in. After its
endpoint and lifecycle have been reviewed independently, select it explicitly:

```bash
aigw account edit <account> --openai-url <reviewed-proxy-base-url>
aigw check
```

This updates the selected Account endpoint and reprojects configured Codex targets;
AIGW does not install, start, stop, configure, or silently fall back to the
proxy. Restore the reviewed team endpoint with
`aigw config import manifest.toml --replace-account <account>`.

Interactive setup prompts once for each missing Account Token, validates every
Account before writing, imports the full profile matrix, and applies the
recommended default and client routes. AIGW stores Tokens in the selected system
secret backend. When Codex is configured, its native login also binds the active
credential into the one admitted Codex home; manifest setup refuses multiple
auto-managed Codex targets because partial native login cannot be rolled back.
`--token-stdin` is intentionally limited to a one-Account manifest; a
multi-Account manifest must use hidden interactive prompts or pre-provisioned
environment secrets.

For each Claude Account, setup makes one bounded, no-session-persistence
model request when Claude is discoverable; otherwise it uses an authenticated
models probe. Codex validation uses the models probe. If an upstream omits that
probe, make Claude discoverable before setting up its Claude Profile.

### Already configured machine

```bash
aigw config import manifest.toml
aigw rotate <account> # only when that Account still lacks a Token
aigw check
```

`setup --from` is first-configuration only. Import remains non-destructive and
never reads, writes, or deletes Account Tokens.

Import also does not delete a local Profile merely because a newer team
manifest omits it. After reviewing that it is not selected by any Route, remove
an obsolete local entry explicitly with `aigw profile remove <profile>`.

Use `aigw catalog --all` for read-only inventory inspection. Catalog results are
not admission evidence and do not automatically change a route. Enable only
clients that are actually used; every new client requires its own admission
record.

For user-authorized end-to-end evidence:

```bash
aigw verify --for all
aigw rollback
aigw rollback --last-change
```

Verification consumes one minimal request per selected client. It writes a
secret-free checkpoint only after both routes succeed. Rollback restores
AIGW-managed configuration only and never manages desktop-client lifecycle.

## Release artifacts

A release must contain this complete 15-artifact matrix:

| Platform | Native package | Portable package |
| --- | --- | --- |
| macOS | Universal `.pkg` | `darwin_amd64`, `darwin_arm64` archives |
| Linux | `amd64` and `arm64` `.deb` and `.rpm` | `amd64` and `arm64` archives |
| Windows | `amd64` and `arm64` `.msi` | `amd64` and `arm64` archives |

The remaining artifacts are `checksums.txt` and an SPDX SBOM. The release
pipeline fails rather than publishing a partial matrix. `checksums.txt` must
contain exactly one SHA-256 record for each non-checksum artifact.

RC artifacts require the complete matrix, checksum/SBOM verification, package
layout checks, portable installer contracts, and cross-platform architecture
proof. A managed Windows native runner adds runtime evidence when available but
does not block an RC; it remains mandatory for GA. macOS native package
acceptance runs only on a disposable APFS target and an isolated local account;
it is additive until its protected runner has an approved dedicated credential.
RC artifacts do not claim signing or notarization. GA release requirements are
defined in [Release evidence](../operations/release-readiness.md).

## Updates

Portable installers copy bundled files only. Formal releases retain the exact
15-artifact matrix: platform packages, `checksums.txt`, and an SPDX SBOM.
GitLab and GitHub are independent forge planes that preserve separate commit
and signed-tag provenance. They are equal update peers: when both are
reachable, their latest tag and current-platform artifact bytes must agree;
when one is unreachable, the remaining reachable peer may provide the update.
A checksum, metadata, authorization, downgrade, or redirect failure is
terminal and must not be hidden by mixing provider data.

After installation, `aigw update` uses the sources embedded by the publishing
pipeline. A source build may configure both independently:

```bash
export AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example.com
export AIGW_GITLAB_RELEASE_REPOSITORY=group/aigw-cli
export AIGW_GITHUB_RELEASE_ORIGIN=https://github.com
export AIGW_GITHUB_RELEASE_REPOSITORY=owner/aigw-cli
```

For offline acceptance, transfer one reviewed platform-matching portable archive
with its validating `checksums.txt` record, rather than a loose executable or
source checkout. Then invoke the explicit candidate path to preserve the normal
atomic replace and single-binary rollback behavior:

```bash
aigw update --candidate /secure-transfer/aigw_0.1.0-candidate.1_darwin_arm64.tar.gz \
  --checksums /secure-transfer/checksums.txt
```

The candidate path never contacts a forge. Remote updates never bypass malformed
metadata, missing artifacts, version conflicts, checksum failures, or redirect
violations. `GITLAB_TOKEN` is an HTTPS-only GitLab API path; GitHub tokens are
ephemeral environment credentials. For a private `github.com` release, AIGW may
also invoke an already-authenticated local `gh` client after the anonymous API
returns its intentional 404 response; it never reads, exports, or persists the
underlying credential. Native channels delegate program updates and rollback to
their platform package manager.

## CI secret boundary

CI can use the read-only environment secret backend:

```bash
export AIGW_SECRET_BACKEND=env
export AIGW_TOKEN_TEAM_GATEWAY=masked-team-token
aigw setup --from manifests/example.toml
```

This backend validates pre-provisioned Tokens but cannot write or delete them.
