# Team Rollout

## Maintainer workflow

Publish a reviewed, token-free account and profile manifest, such as
the repository deployment manifest
[`team/team-profiles.toml`](../team/team-profiles.toml). Use
[`examples/team-profiles.toml`](../examples/team-profiles.toml) as the
provider-neutral format example. A manifest may contain account metadata,
endpoints, profiles, models, purposes, and a recommended default plus optional
recommended client routes. It must not contain tokens, adapter state, or
machine paths.

Recommended client routes require team manifest v3. Current clients continue to
accept v2 manifests without that table; older clients reject v3 and must be
updated before rollout rather than partially applying it.

```bash
aigw config export > team-profiles.toml
```

Exported client routes are recommendations. Import and setup fill only a
member's missing route selections; they do not replace an existing personal
Claude or Codex route.

Import is idempotent only for semantically identical names. A conflict stops
before mutation. A reviewed operator can replace public metadata explicitly:

```bash
aigw config import team-profiles.toml --replace-account dmxapi
aigw config import team-profiles.toml --replace-profile dmxapi-gpt-5.6-terra
```

Those switches never read, copy, delete, or replace a member's system token.

## Member workflow

### New machine

Review each Account's public endpoint and optional
`codex_responses_storage` requirement. Profiles, models, and recommended routes
are maintained team metadata; Tokens never belong in this file. The repository
deployment manifest includes a loopback compatibility endpoint; keep it only
when that listener exists on the member's machine.

```bash
aigw setup --from team-profiles.toml
aigw check
```

Interactive setup prompts once for each missing Account Token, validates every
Account before writing, imports the full profile matrix, and applies the
recommended default and client routes. AIGW stores Tokens in the selected system
secret backend. When Codex is configured, its native login also binds the active
credential into the one admitted Codex home; team setup refuses multiple
auto-managed Codex targets because partial native login cannot be rolled back.
`--token-stdin` is intentionally limited to a one-Account manifest; a
multi-Account manifest must use hidden interactive prompts or pre-provisioned
environment secrets.

If Claude is discoverable, validation makes one bounded, no-session-persistence
minimal model request per Claude Account. Otherwise setup uses a strict
authenticated models probe; install or make Claude discoverable first when a
provider does not expose that probe. Codex validation uses the models endpoint.

### Already configured machine

```bash
aigw config import team-profiles.toml
aigw rotate <account> # only when that Account still lacks a Token
aigw check
```

`setup --from` is first-configuration only. Import remains non-destructive and
never reads, writes, or deletes Account Tokens.

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
defined in [Release evidence](release-readiness.md).

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
aigw setup --from examples/team-profiles.toml
```

This backend validates pre-provisioned Tokens but cannot write or delete them.
