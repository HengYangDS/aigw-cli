# AIGW CLI

| GitLab metadata | Value |
| --- | --- |
| **Project Name** | `AIGW CLI` |
| **Stable repository Path** | `aigw-cli` |

AIGW CLI is a local-first, cross-platform control plane for AI provider
accounts, credentials, runtime profiles, client routes, and Claude/Codex
projections. It does not run a gateway, listen on a port, relay API traffic, or
own Codex conversation state.

## What it manages

- **Accounts**: one provider boundary, verified endpoint(s), and one operating-
  system secret slot.
- **Profiles**: an admitted `account + client + model` daily choice.
- **Routes**: default, Claude, and Codex selections.
- **Adapters**: narrow local projections for Claude and Codex.

Codex Desktop owns conversation transcripts and per-conversation model choice.
AIGW owns only its marked provider projection and native credential binding.
Codex DMX Proxy owns Responses replay compatibility and its own listener.

## Install

AIGW has three equivalent, checksum-first distribution paths:

1. **GitLab primary release** — the organization’s formal release source.
2. **GitHub mirror** — an independent mirror of the exact verified asset set.
3. **Verified local candidate** — a complete, extracted artifact directory for
   offline installation and acceptance testing.

A formal release is the exact 15-artifact matrix: platform packages,
`checksums.txt`, and an SPDX SBOM. Verify the archive you will install against
`checksums.txt`. A verified local candidate is an extracted, complete artifact
directory with the same release files retained together. A source checkout, an
arbitrary binary, and a Git tag alone are not release artifacts.

| Platform | Native package | Portable package |
| --- | --- | --- |
| macOS | `aigw_<version>_darwin_universal.pkg` | `aigw_<version>_darwin_{amd64,arm64}.tar.gz` |
| Linux | `aigw_<version>_linux_{amd64,arm64}.{deb,rpm}` | `aigw_<version>_linux_{amd64,arm64}.tar.gz` |
| Windows | `aigw_<version>_windows_{amd64,arm64}.msi` | `aigw_<version>_windows_{amd64,arm64}.zip` |

From an extracted portable archive:

```bash
sh install.sh
```

```powershell
.\install.ps1
```

Portable installers copy only the bundled executable. They do not access the
network, retrieve a release, or read release credentials. They install under
the current user and retain one immediate predecessor for offline program
rollback. Native packages own only their package-managed files.

## Quick start

Interactive setup creates the first account, profile, and route without
assuming a provider, endpoint, model, or token:

```bash
aigw setup
```

For a reviewed, token-free team manifest:

```bash
aigw config import team-profiles.toml
aigw rotate team-gateway
aigw use gpt-5.6-terra --for codex
aigw use claude-fable-5 --for claude
aigw check
```

Import is non-destructive by default. A same-named account or profile must
match exactly or the import stops before any mutation. An explicit reviewed
replacement changes public metadata only; it never reads, writes, or deletes a
stored token:

```bash
aigw config import team-profiles.toml --replace-account team-gateway
aigw config import team-profiles.toml --replace-profile gpt-5.6-terra
```

## Daily operations

```bash
aigw                         # current routes and readiness
aigw use [profile]           # choose a profile
aigw route list              # inspect route overrides
aigw rotate [account]        # replace one account token
aigw catalog [--all|--json]  # inspect an account model inventory
aigw check                   # configuration and endpoint health
aigw sync --dry-run --json   # inspect all Codex projection actions
aigw sync                    # atomically reconcile marked projections
aigw verify --for all        # opt-in minimal real model requests
aigw rollback                # restore the latest verified configuration
aigw update                  # update the installed program
aigw update --rollback       # offline portable-program rollback only
```

`aigw test` is a bounded connectivity and authentication check. `aigw verify`
makes a minimal real request and consumes provider quota only when explicitly
run. `aigw rollback` restores AIGW-managed configuration; it never restarts a
client. `aigw update --rollback` swaps only the portable executable and its one
local predecessor; native-package rollback belongs to the platform package
manager.

## Client boundaries

Claude uses an AIGW-owned shim in AIGW's private data directory. Its account
token becomes `ANTHROPIC_AUTH_TOKEN` only in the Claude process it launches.
Codex receives only AIGW-marked top-level `model`, `model_provider`, and a
provider projection. `aigw sync` performs an all-target transaction and does
not start, stop, restart, reload, or alter a Codex conversation.

For a drifted target, inspect the plan first:

```bash
aigw sync --dry-run --json
aigw sync
```

## Update sources

A released binary embeds GitLab as its primary source and may embed a GitHub
mirror. `aigw update` uses GitLab first, then uses GitHub **only** when the
primary source is unavailable. A malformed release, a tag/version conflict, a
missing asset, or any checksum failure is terminal: AIGW does not switch
sources after an integrity or provenance failure.

A locally built binary has no implicit vendor endpoint. For a local verified
candidate, set `AIGW_LOCAL_CANDIDATE` to the extracted artifact directory. The
directory must contain exactly one portable archive for the current platform
and `checksums.txt` that validates that archive:

```bash
export AIGW_LOCAL_CANDIDATE=/secure/path/to/aigw-0.1.0-rc.1
AIGW_LOCAL_CANDIDATE="$AIGW_LOCAL_CANDIDATE" aigw update
```

For source builds that must test remote behavior, configure the GitLab primary
and optional GitHub mirror explicitly:

```bash
export AIGW_RELEASE_HOST=https://gitlab.example.com
export AIGW_RELEASE_PROJECT=group/aigw-cli
export AIGW_RELEASE_MIRROR_HOST=https://github.com
export AIGW_RELEASE_MIRROR_PROJECT=owner/aigw-cli
```

AIGW prefers authenticated `glab` for GitLab. If `glab` is unavailable, an
explicit `GITLAB_TOKEN` may be used for an HTTPS GitLab API fallback. GitHub
mirror retrieval uses its public release metadata and assets; a private mirror
must be exposed through a repository-scoped release path supported by the
organization before it is enabled. No token is stored by the updater, passed on
a command line, or forwarded across hosts. Every downloaded artifact is
checksum-verified before replacement.

## Product boundaries

AIGW is a control plane, not a proxy or gateway. It does not own or modify
Codex JSONL, SQLite, archived conversations, model metadata, or a DMX Proxy
process. A future organizational gateway is simply a verified HTTPS account
endpoint from AIGW's perspective.

GitLab and GitHub are independent source-verification and release planes.
GitLab retains canonical source history; GitHub independently verifies the
same source tree and may publish an identical recovery release. Neither plane
uses the other provider's green pipeline as substitute evidence.

## Documentation and contribution

- [Concepts](docs/concepts.md)
- [Security model](docs/security.md)
- [Model strategy](docs/model-strategy.md)
- [Adapter admission](docs/adapter-admission.md)
- [Team rollout](docs/team-rollout.md)
- [Release evidence](docs/release-readiness.md)
- [Contribution guide](CONTRIBUTING.md)
- [Documentation root](docs/README.md)
- [Change and release policy](docs/governance/change-and-release-policy.md)

## Verify a source checkout

```bash
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd internal tools)"
sh scripts/check-governance.sh
python3 scripts/check-markdown-presentation.py
sh scripts/test-changelog.sh
AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh 0.1.0-rc.1 dist
sh scripts/test-release-package-layout.sh dist 0.1.0-rc.1
```

A source tag is not proof of published assets, native-platform acceptance, or
GA signing. The evidence boundaries are defined in
[release-readiness.md](docs/release-readiness.md).
