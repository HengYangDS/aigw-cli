---
subject: aigw:repository-tooling
role: explanation
state: canonical
relations:
  canonical_for: AIGW repository tool ownership
---

# Repository Tooling

Repository-only executables follow the same ontology: [CI](../../tools/ci/),
[coverage](../../tools/coverage/), [Forge operations](../../tools/forge/), and
[releases](../../tools/release/) own cross-cutting repository concerns, while client-specific verification is nested
under its client owner. [Codex catalog verification](../../tools/codex/catalogcheck/) owns catalog acceptance,
measurements and reporting. It consumes the same [catalog projection](../../internal/adapter/codex/modelcatalog/)
document transformation as product projection, not a second parser or alias
builder. That document owner preserves all client metadata and adds only
uniquely established aliases. [Codex integration](../../internal/adapter/codex/) owns isolated client reads,
executable identity, projection state and transactional file ownership; it does
not carry repository acceptance verdicts.

Native product journeys belong to [release acceptance](../../tools/release/): they exercise built artifacts,
credential backends, client projections, upgrades, rollback and uninstall. CI
schedules those tests but does not own their fixtures. [Release identity](../../tools/release/identity/)
reads and validates the [canonical version](../../VERSION) once for CI admission, release
construction and native journeys; malformed SemVer fails before execution.
Package-local test file readers do not expose a shared test-support API.

CI projection binds the shared accepted `dev` and release `main` roles in its
existing CUE owner and reads no repository branch policy, so a competing branch
policy cannot produce a projection.
Release identity also owns Changelog chronology, exact tag-to-HEAD binding,
and release-epoch lookup. CI invokes that owner directly; no generic repository
command layer or duplicated Changelog parser remains.

[Forge commands](../../tools/forge/main.go), [Git-object verification](../../tools/forge/provenance.go),
and [peer publication](../../tools/forge/project.go) have distinct owners.
Publication calls the same typed commit verifier as the read-only command.
That verifier projects ETHOS's fixed commit grammar and consumes optional scope
restrictions and SSH-signing requirements from the resolved source commit.
Identity metadata also comes from that object; unrelated checkouts and
uncommitted files cannot replace it.
Peer selection cannot change that object or its verification authority.

Release construction, readiness, publication and upgrade consume the existing
SemVer library's strict parser, not separate version grammars. Publication
validates its tag once and carries the parsed identity into asset selection and
stability classification. Prerelease status comes from the prerelease field,
never punctuation or channel-like text in build metadata. Native signing
admission remains a separate readiness responsibility; a valid version alone
does not establish release readiness. CUE runs tag-source verification and
requires that same admission before each peer verifies its published artifacts.
The operator constructs and signs one matrix; each peer independently receives
and verifies those bytes using public trust, without hosted signing keys. Document formatting,
structure and links have their native quality gates; prose is not release
evidence.

Native acceptance consumes the upgrade archive contract rather than parsing a
GoReleaser binary inventory or duplicating extraction. The declared dependency
is one-way: release construction calls the [archive target contract](../../internal/program/artifact/identity.go) to
verify and extract the same bytes that product updates consume. Runtime code
does not import repository tools. Both built and published inputs use that
reader; acceptance owns scratch, never the input artifact directory.

Construction owns validation of embedded release sources. The standalone
source check and build admission use the same [release-source validator](../../internal/program/source.go)
before running tools. Each Forge is optional; a selected source requires an
admitted origin and complete repository path together. HTTPS is required except
for localhost, `.test` names, and literal loopback, private, or link-local IP
addresses, which may use HTTP. GitLab permits nested namespaces, while GitHub
requires an owner/repository pair. Runtime endpoint overrides remain the upgrade
owner's responsibility, not a dependency of construction.

An origin contains only a scheme and nonempty hostname with an optional port
and root slash. Native URL reconstruction rejects credentials, paths and even
empty query or fragment markers. Repository coordinates are unescaped relative
paths with a namespace; native path validation and URL escaping must preserve
the input exactly. Encoded separators cannot change its routing meaning.
Both boundaries use the standard library rather than separate segment scanners.
Embedded metadata and runtime source overrides share this origin admission rule.
The direct-request `GITLAB_TOKEN` fallback separately requires HTTPS; that
credential-transport rule does not invalidate admitted private HTTP release metadata.
