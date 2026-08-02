# AIGW CLI

| GitLab metadata | Value |
| --- | --- |
| **Project Name** | `AIGW CLI` |
| **Stable repository Path** | `aigw-cli` |

AIGW CLI is a local-first, cross-platform control plane for enterprise teams
using reviewed third-party AI services. It manages provider Accounts,
system-stored credentials, model Profiles, Routes, and explicit Claude/Codex
client integration. It is distributed under the [MIT](LICENSE) License. It
does not run a gateway, listen on a port, relay API traffic, or own Codex
conversation state.

## Start with the task

| I want to… | Run | Then |
| --- | --- | --- |
| Connect my first service | `aigw setup` | `aigw check` |
| Use a different model profile | `aigw use <profile>` | `aigw check` |
| Know what is active | `aigw` | Follow its one **Next** command |
| Recover a local client integration | `aigw doctor` | Run its recommended action |
| Join a team with a reviewed manifest | `aigw config import manifest.toml` | `aigw rotate <account>` |

The everyday path is deliberately small: **setup → use → check**. AIGW
shows the current selection, readiness, and one safe next action; detailed
object management stays under explicit advanced commands.

## Install

AIGW has three checksum-first distribution paths:

1. **GitLab release** — an independent organization forge plane.
2. **GitHub release** — an independent peer forge plane.
3. **Verified local candidate** — a reviewed portable archive and its checksum
   manifest for offline installation and acceptance testing.

The source code, documentation, and distributed release files are available
under the permissive [MIT License](LICENSE). Third-party dependencies retain
their own licenses, as identified by the release SPDX SBOM.

AIGW is installed from verified release artifacts; it is not published as an
importable Go library. The short module declaration in `go.mod` is a
non-fetchable build identity, deliberately independent of either Forge,
organization, maintainer, and local filesystem layout.

GitLab and GitHub are independent, complete forge planes. They publish the same
versioned release matrix and use the same release filenames, checksums, and SBOM
content. Their commit histories, signed tags, and provider identities remain
separate provenance records. A released binary treats GitLab and GitHub as
equal update peers: when both are reachable, their latest tag and current
platform artifact bytes must agree before installation. When only one peer is
reachable, that peer may supply the update. Integrity, authentication,
metadata, version, and redirect failures remain terminal; AIGW never mixes
files across forges.

A formal release is the exact 15-artifact matrix: platform packages,
`checksums.txt`, and an SPDX SBOM. Verify the archive you will install against
`checksums.txt`. A verified local candidate is one reviewed archive plus the
matching checksum manifest. A source checkout, arbitrary binary, and Git tag
alone are not release artifacts.

| Platform | Recommended package | Portable package |
| --- | --- | --- |
| macOS Intel / Apple Silicon | `aigw_<version>_darwin_universal.pkg` | `darwin_amd64` / `darwin_arm64` archives |
| Linux x86-64 | `.deb` or `.rpm` | `linux_amd64.tar.gz` |
| Linux ARM64 | `.deb` or `.rpm` | `linux_arm64.tar.gz` |
| Windows x86-64 | `.msi` | `windows_amd64.zip` |
| Windows ARM64 | `.msi` | `windows_arm64.zip` |

From an extracted portable archive, run the bundled installer:

```bash
sh install.sh
```

```powershell
.\install.ps1
```

Portable installers only copy the bundled executable. They do not retrieve a
release, store a credential, register a service, or configure a proxy.

## First connection

Interactive setup is provider-neutral. It creates one Account, one profile,
and one secure Token slot without assuming a gateway, model, or provider:

```bash
aigw setup
```

For a new machine with a reviewed, token-free configuration manifest, review
only the Account endpoints and run one command. AIGW imports every profile,
prompts once for each missing Account Token, validates the credentials, and
keeps Tokens out of the manifest and local config:

```bash
aigw setup --from manifest.toml
```

Start from the deliberately fictitious, provider-neutral
[`manifests/example.toml`](manifests/example.toml). Real provider endpoints,
model IDs, route recommendations, and diagnostic policies belong to the
adopting team's separately reviewed manifest, not the product repository.

For a Claude Profile, setup uses one bounded, no-session-persistence model
request when a Claude executable is discoverable. Otherwise it uses an
authenticated models probe. For a Codex Profile, setup uses the authenticated
models probe. If an upstream omits that probe, make Claude discoverable before
setting up its Claude Profile.

`setup --from` and recommended client routes use configuration manifest v3.
Older AIGW clients fail closed on v3 and must be updated before this rollout
path is used.

On an already configured machine, merge public metadata without touching
existing Tokens:

```bash
aigw config import manifest.toml
```

Import is non-destructive. A same-named Account or profile must match exactly
or the import stops before mutation. Explicit replacement changes public
metadata only; it never reads, writes, or deletes the Account Token.

## Every day

```bash
aigw                         # active profile, client readiness, one next step
aigw use [profile]           # choose a profile
aigw rotate [account]        # replace one account token
aigw check                   # configuration, client, and endpoint health
aigw doctor                  # detailed diagnosis and one recovery action
aigw repair                  # bounded client discovery and reconciliation
aigw repair --dry-run --json # preview repair without writing or binding auth
aigw profile rename [old] [new] # rename a profile; updates route references
aigw account rename [old] [new] # rename an account; two-phase credential migration
```

`aigw test` is a bounded connectivity and authentication check. `aigw verify`
makes a minimal real request and consumes provider quota only when explicitly
run. `aigw rollback` restores AIGW-managed configuration and never restarts a
client. `aigw update --rollback` restores only the portable program's immediate
predecessor; native-package rollback belongs to the platform package manager.

## What AIGW manages

- **Accounts**: one provider boundary, verified endpoint(s), and one
  operating-system secret slot.
- **Profiles**: an admitted `account + client + model` daily choice.
- **Routes**: default, Claude, and Codex selections.
- **Adapters**: bounded client integrations for Claude and Codex.

The admitted client set is intentionally small: Claude Code and the Codex Home
shared by Codex CLI and Codex Desktop. Setup configures only an admitted client
whose required executable and configuration surface are discoverable; a client
that is not installed remains untouched and is reported as not configured.
Hermes and other future clients require their own admitted Adapter and are not
configured by the current release.

Claude receives `ANTHROPIC_AUTH_TOKEN` only in the process launched through the
AIGW-owned launcher. Codex receives only AIGW-marked configuration and native
credential binding. The integrations do not write into one another's owned
surfaces.

Codex Desktop owns conversation transcripts and each conversation's model
choice. AIGW owns only its marked provider projection and native credential
binding. If a loopback compatibility layer is present, it remains external:
Codex requests use that listener, so it must be available for the selected
route to work. AIGW does not start, stop, configure, or diagnose its service
lifecycle.

### Codex ownership boundary

Codex CLI and Codex Desktop share the Codex Home configuration. AIGW projects
to configured Codex homes; the default target is `~/.codex/config.toml`, and an
operator may explicitly configure another Codex Home target. AIGW marks and
reconciles only its provider/model block and sidecar. It does not create a
second Desktop adapter or manage Desktop-only GUI settings.

Codex owns every existing conversation's model, transcript, JSONL, SQLite, and
runtime metadata. IDE integrations and other clients own their own configuration
and lifecycle. They are not discovered, diagnosed, repaired, or controlled by
AIGW. To use an external Responses compatibility service, select its HTTP
endpoint in an Account; AIGW does not install or manage that service.

## Update sources

A formal package reads its two official update peers from protected release
execution inputs. The tracked `.config/release/forge-sources.env` is only a
fictitious shape fixture; it is never embedded. Both release planes validate
and embed the explicitly supplied tuple set. A direct development `go build`
has no implicit vendor endpoint:

```bash
export AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example.com
export AIGW_GITLAB_RELEASE_REPOSITORY=group/aigw-cli
export AIGW_GITHUB_RELEASE_ORIGIN=https://github.com
export AIGW_GITHUB_RELEASE_REPOSITORY=owner/aigw-cli
```

A verified local candidate is deliberately separate from a release source: it is
one reviewed archive plus its checksum manifest, not a tag or published release.
It never opens a network connection:

```bash
aigw update --candidate /secure-transfer/aigw_0.1.0-candidate.1_darwin_arm64.tar.gz \
  --checksums /secure-transfer/checksums.txt
```

AIGW prefers authenticated `glab` for GitLab. If `glab` is unavailable, an
explicit `GITLAB_TOKEN` may be used for an HTTPS GitLab API path. GitHub uses an
optional ephemeral `AIGW_GITHUB_TOKEN`, `GITHUB_TOKEN`, or `GH_TOKEN` for
private releases, in that precedence order. When GitHub intentionally returns
an anonymous 404 for a private release and no environment token is available,
AIGW may use the existing local `gh` authentication path for `github.com` only.
It never reads, exports, stores, or forwards a `gh` credential. Every downloaded
artifact is checksum-verified before replacement.

## When you need more

| Need | Start here |
| --- | --- |
| Core Account, Profile, Route, and Adapter concepts | [Concepts](docs/concepts/README.md) |
| Secure local credential and client boundaries | [Security model](docs/governance/security.md) |
| Configuration manifest and release workflow | [Team rollout](docs/guides/team-rollout.md) |
| Terminal navigation and presentation rules | [Terminal experience contract](docs/governance/terminal-experience-contract.md) |
| Control-plane and proxy boundary | [Authority and projection boundary](docs/architecture/authority-and-projection-boundary.md) |
| Full documentation map | [Documentation root](docs/README.md) |
| Contributing and verification | [Contribution guide](CONTRIBUTING.md) |

## Verify a source checkout

```bash
go run ./tools/architecture --root .
sh scripts/checks/governance/check-module-identity.sh
go run ./tools/coveragegate --race
go vet ./...
sh scripts/checks/quality/check-static-analysis.sh
sh scripts/checks/governance/check-portability.sh
sh scripts/tests/governance/test-portability.sh
test -z "$(gofmt -l cmd internal tools)"
sh scripts/checks/governance/check-governance.sh
AIGW_GITLAB_AUTHOR_EMAIL='<release actor email>' AIGW_GITLAB_ALLOWED_SIGNERS='<path>' sh scripts/checks/forge/check-commit-provenance.sh . gitlab
sh scripts/tests/forge/test-commit-provenance.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/tests/forge/test-replay-history.py
AIGW_TAG_NAMESPACE_FORGE='<local|gitlab|github>' AIGW_GITLAB_ALLOWED_SIGNERS='<path>' AIGW_GITHUB_ALLOWED_SIGNERS='<path>' sh scripts/checks/forge/check-tag-namespace.sh
python3 scripts/checks/governance/check-markdown-presentation.py
python3 scripts/checks/governance/check-text-layout.py
sh scripts/tests/governance/test-text-layout.sh
sh scripts/tests/governance/test-changelog.sh
export GOTOOLCHAIN=go1.26.5
sh scripts/checks/release/check-release-toolchain.sh
version=$(git describe --tags --exact-match | sed 's/^v//')
SOURCE_DATE_EPOCH=$(sh scripts/release/lib/release-source-date-epoch.sh "$version") \
  AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/release/build/package.sh "$version" dist
sh scripts/tests/release/test-release-package-layout.sh dist "$version"
SOURCE_DATE_EPOCH=$(sh scripts/release/lib/release-source-date-epoch.sh "$version") \
  sh scripts/tests/release/test-release-reproducibility.sh "$version"
```

A source tag is not proof of published assets, native-platform acceptance, or
GA signing. The evidence boundaries are defined in
[release-readiness.md](docs/operations/release-readiness.md).
