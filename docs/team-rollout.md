# Team Rollout

## Maintainer workflow

Publish a reviewed, token-free account and profile manifest, such as
[`examples/team-profiles.toml`](../examples/team-profiles.toml). It may contain
account metadata, endpoints, profiles, models, purposes, and a recommended
default. It must not contain tokens, personal route overrides, adapter state, or
machine paths.

```bash
aigw config import team-profiles.toml
aigw config export
```

Import is idempotent only for semantically identical names. A conflict stops
before mutation. A reviewed operator can replace public metadata explicitly:

```bash
aigw config import team-profiles.toml --replace-account team-gateway
aigw config import team-profiles.toml --replace-profile gpt-5.6-terra
```

Those switches never read, copy, delete, or replace a member's system token.

## Member workflow

```bash
aigw config import team-profiles.toml
aigw rotate team-gateway
aigw use gpt-5.6-terra --for codex
aigw use claude-fable-5 --for claude
aigw check
```

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

RC artifacts require current structural and installation evidence; they do not
claim signing or notarization. GA release requirements are defined in
[Release evidence](release-readiness.md).

## Updates

Portable installers copy bundled files only. Formal releases retain the exact
15-artifact matrix: platform packages, `checksums.txt`, and an SPDX SBOM.
GitLab and GitHub are complete independent release planes. Each publishes its
own tag pipeline, full artifact matrix, checksum manifest, SPDX SBOM, and draft
release. The two planes must converge on exact asset bytes before either is
admitted as the other's fallback.

After installation, `aigw update` uses the source embedded by the publishing
pipeline. A source build may configure either provider as primary and the other
as fallback explicitly:

```bash
export AIGW_RELEASE_PROVIDER=gitlab
export AIGW_RELEASE_HOST=https://gitlab.example.com
export AIGW_RELEASE_PROJECT=group/aigw-cli
export AIGW_RELEASE_MIRROR_PROVIDER=github
export AIGW_RELEASE_MIRROR_HOST=https://github.com
export AIGW_RELEASE_MIRROR_PROJECT=owner/aigw-cli
```

For offline acceptance, transfer a complete extracted artifact directory rather
than a loose executable or source checkout. It must contain exactly one
platform-matching portable archive and a validating `checksums.txt` record.
Then invoke the explicit candidate path to preserve the normal atomic replace
and single-binary rollback behavior:

```bash
export AIGW_LOCAL_CANDIDATE=/secure-transfer/aigw-0.1.0-rc.1
AIGW_LOCAL_CANDIDATE="$AIGW_LOCAL_CANDIDATE" aigw update
```

The updater may use its configured fallback only after the embedded primary is
unavailable. It never switches providers to bypass malformed metadata, missing
assets, a version conflict, or a checksum failure. `GITLAB_TOKEN` is an HTTPS
GitLab fallback only. Native channels delegate program updates and rollback to
their platform package manager.

## CI secret boundary

CI can use the read-only environment secret backend:

```bash
export AIGW_SECRET_BACKEND=env
export AIGW_TOKEN_TEAM_GATEWAY=masked-ci-token
```

This backend validates a pre-provisioned token but cannot write or delete it.
