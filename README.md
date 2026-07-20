# AIGW CLI

| GitLab metadata | Value |
| --- | --- |
| **Project Name** | `AIGW CLI` |
| **Stable repository Path** | `aigw-cli` |

AIGW CLI is a local-first, cross-platform control plane for AI provider
accounts, system-stored credentials, model profiles, client routes, and
Claude/Codex projections. It is distributed under the [MIT](LICENSE) License.
It does not run a gateway, listen on a port, relay API traffic, or own Codex
conversation state.

## Start with the task

| I want to… | Run | Then |
| --- | --- | --- |
| Connect my first service | `aigw setup` | `aigw check` |
| Use a different model profile | `aigw use <profile>` | `aigw check` |
| Know what is active | `aigw` | Follow its one **Next** command |
| Recover a local client integration | `aigw doctor` | Run its recommended action |
| Join a team with a reviewed manifest | `aigw config import team-profiles.toml` | `aigw rotate <account>` |

The everyday path is deliberately small: **setup → use → check**. AIGW shows
current selection, readiness, and one safe next action; detailed object
management stays under explicit advanced commands.

## Install

AIGW has three checksum-first distribution paths:

1. **GitLab release** — an independent organization forge plane.
2. **GitHub release** — an independent peer forge plane.
3. **Verified local candidate** — a reviewed portable archive and its checksum
   manifest for offline installation and acceptance testing.

The source code, documentation, and distributed release files are available
under the permissive [MIT License](LICENSE). Third-party dependencies retain
their own licenses, as identified by the release SPDX SBOM.

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

For a reviewed, token-free team manifest:

```bash
aigw config import team-profiles.toml
aigw rotate team-gateway
aigw use gpt-5.6-terra --for codex
aigw use claude-fable-5 --for claude
aigw check
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
aigw route doctor            # inspect host-route ownership; no probes or writes
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
- **Adapters**: narrow local projections for Claude and Codex.

Claude receives `ANTHROPIC_AUTH_TOKEN` only in the process launched by AIGW's
private shim. Codex receives only AIGW-marked configuration and native
credential binding. The tools do not write into one another's directories.

Codex Desktop owns conversation transcripts and each conversation's model
choice. AIGW owns only its marked provider projection and native credential
binding. If a loopback compatibility layer is present, it remains external:
Codex requests use that listener, so it must be available for the selected
route to work. AIGW does not start, stop, configure, or diagnose its service
lifecycle.

### Host-specific Codex routing

Codex is not one homogeneous host surface. On macOS, AIGW classifies the
following surfaces before it plans a projection:

| Surface | Persistent default | AIGW behavior |
| --- | --- | --- |
| Ordinary standalone Codex CLI | AIGW | AIGW may manage its full provider/model selection. Generic `setup` and `repair` adopt only this surface. |
| ChatGPT Desktop | AIGW default configuration for later/new work | Desktop alone owns each existing conversation's model and transcript; AIGW never edits conversation state. |
| PyCharm Codex | JetBrains AI | Classified for diagnosis and excluded from AIGW target adoption. |
| JetBrains Air | JetBrains AI | Exact standalone copies remain external host mirrors. AIGW may stage only an explicit, reversible namespaced fallback. |
| Junie CLI | JetBrains AI through Junie Account / JetBrains Account | Observed as a JetBrains surface, never admitted as a Codex target or executed by route diagnosis. |

Use `aigw route doctor --json` for a local, secret-free ownership report. It
does not run Codex, Junie, or an IDE; read credentials; contact an endpoint; or
report configuration bodies, paths, sessions, or billing as known facts.
For Air, the same read-only report includes bounded recovery lifecycle health
and stable reason codes for missing, invalid, permission-unsafe, or unexpected
private recovery state. It never creates recovery storage or exposes its paths,
complete digests, quarantine bytes, or case preimages.

An `external-host-mirror` is a healthy JetBrains-owned copy whose exact managed
projection matches the current attributed standalone projection. It is not an
AIGW target and receives no mutation guidance. Optional bounded forwarding
evidence is available without credentials or writes:

```bash
aigw route attest air --json
```

For an ordinary ownership conflict, run `aigw repair --dry-run --json` before
any mutation. The ADR-0003 state `recoverable-stale-full-selection` instead
uses its dedicated recovery preview:

```bash
aigw route recover air --dry-run --json
aigw route recover air --confirm-host-idle
```

Recovery accepts only that exact AIGW-owned mismatch. It removes the marked
AIGW full selection, stale sidecar, and Air's explicit AIGW target membership;
it does not fabricate a JetBrains `model` or `model_provider` selection,
authenticate a client, or touch conversation state. A later Air UI session is
still the authority for proving JetBrains authentication and user-visible
behavior.

The separate `orphaned-exact-full-selection` state is a sidecar-absent exact
generated projection that cannot be proven equal to the current standalone
reference. Recover it only through its deterministic case and private
quarantine:

```bash
aigw route recover-orphan air --dry-run --json
aigw route recover-orphan air --case-id <id> --confirm-host-idle --ack-unset-external-selection
aigw route settle air --case-id <id> --dry-run --json
aigw route settle air --case-id <id>
```

Recovery writes no replacement provider or model selection. It leaves an
unset external baseline and waits for a separately observed host roundtrip.
Settlement changes only the private recovery ledger and quarantine; it never
writes Air. Neither attestation nor settlement proves login, authentication,
quota, billing, terminal success, or a user-visible reply.

Air remains JetBrains AI by default. Its opt-in fallback has a separate,
deliberate path:

```bash
aigw route fallback air --dry-run --json
aigw route fallback air --confirm-host-idle
aigw route restore air --dry-run --json
aigw route restore air --confirm-host-idle
aigw route recover air --dry-run --json
aigw route recover air --confirm-host-idle
```

The dry runs do not write configuration, bind authentication, or start a
client. The apply commands require an operator attestation that Air is idle;
they do not probe, start, stop, or restart Air. Fallback appends an
AIGW-owned namespaced block only. It never changes Air's top-level `model` or
`model_provider`; restore removes only that owned block and preserves the
remaining Air file byte-for-byte. If Air is already selected to AIGW at the
top level, both operations fail closed until its native settings return it to
JetBrains AI. `route recover air` is not a fallback operation: it is admitted
only for the reported stale full-selection/fallback-sidecar mismatch and
returns Air to an unselected external baseline.

## Update sources

A formal package reads its two official update peers from the tracked,
credential-free `packaging/release/forge-sources.env` manifest. Both release
planes validate and embed that same complete tuple set; a conflicting CI or
shell override is rejected. A direct development `go build` has no implicit
vendor endpoint. For an intentional remote test, configure a complete tuple for
either or both peers; never combine individual fields:

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
| Core Account, Profile, Route, and Adapter concepts | [Concepts](docs/concepts.md) |
| Secure local credential and client boundaries | [Security model](docs/security.md) |
| Team manifest and release workflow | [Team rollout](docs/team-rollout.md) |
| Terminal navigation and presentation rules | [Terminal experience contract](docs/governance/terminal-experience-contract.md) |
| Control-plane and proxy boundary | [Authority and projection boundary](docs/architecture/authority-and-projection-boundary.md) |
| Full documentation map | [Documentation root](docs/README.md) |
| Contributing and verification | [Contribution guide](CONTRIBUTING.md) |

## Verify a source checkout

```bash
go test -race ./...
go vet ./...
sh scripts/check-static-analysis.sh
test -z "$(gofmt -l cmd internal tools)"
sh scripts/check-governance.sh
sh scripts/check-tag-namespace.sh
python3 scripts/check-markdown-presentation.py
sh scripts/test-changelog.sh
export GOTOOLCHAIN=go1.25.12
sh scripts/check-release-toolchain.sh
version=$(git describe --tags --exact-match | sed 's/^v//')
SOURCE_DATE_EPOCH=$(sh scripts/release-source-date-epoch.sh "$version") \
  AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh "$version" dist
sh scripts/test-release-package-layout.sh dist "$version"
SOURCE_DATE_EPOCH=$(sh scripts/release-source-date-epoch.sh "$version") \
  sh scripts/test-release-reproducibility.sh "$version"
```

A source tag is not proof of published assets, native-platform acceptance, or
GA signing. The evidence boundaries are defined in
[release-readiness.md](docs/release-readiness.md).
