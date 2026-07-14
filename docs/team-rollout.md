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

Portable installers copy bundled files only. After installation, `aigw update`
uses the release source embedded by the publishing pipeline, or the explicit
portable configuration:

```bash
export AIGW_RELEASE_HOST=https://gitlab.example.com
export AIGW_RELEASE_PROJECT=group/aigw-cli
```

The updater prefers authenticated `glab`; `GITLAB_TOKEN` is an HTTPS fallback
only. It verifies checksums, never persists a token, and keeps one portable
rollback binary. Native channels delegate program updates and rollback to their
platform package manager.

## CI secret boundary

CI can use the read-only environment secret backend:

```bash
export AIGW_SECRET_BACKEND=env
export AIGW_TOKEN_TEAM_GATEWAY=masked-ci-token
```

This backend validates a pre-provisioned token but cannot write or delete it.
