<!--
---
subject: aigw:change-and-release-policy
role: policy
state: canonical
relations:
  canonical_for: change and release governance
---
-->

# Change and Release Policy

## Authority Map

| Meaning                                | Authoritative owner                                                                                                                                            |
| -------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Product version and release chronology | [VERSION](../../VERSION) and [CHANGELOG](../../CHANGELOG.md)                                                                                                   |
| Product commits and annotated tags     | Exact signed local Git objects                                                                                                                                 |
| Change intent and accepted behavior    | [OpenSpec Changes](../../openspec/changes/) and [canonical specs](../../openspec/specs/)                                                                       |
| Go dependency graph                    | [go.mod](../../go.mod) and [go.sum](../../go.sum)                                                                                                              |
| Runtimes and standalone tools          | [mise.toml](../../mise.toml), [mise.lock](../../mise.lock) and native sidecars                                                                                 |
| npm graph                              | [package.json](../../package.json) and [package-lock.json](../../package-lock.json)                                                                            |
| CI topology                            | [CUE](../../.config/ci/pipeline.cue), projected to both peers                                                                                                  |
| Quality and source size                | [Coverage policy](../../.config/checks/coverage/policy.toml), [Go policy](../../.config/checks/go/policy.yml), [SCC budget](../../.config/checks/go/size.toml) |
| Construction and publication           | [Release owner](../../tools/release/) and native packaging policy                                                                                              |

Workflows, Forge displays, installations, caches and IDE indexes are projections.
Root files retain native discovery names; [`.config/checks`](../../.config/checks/) contains native policy
only, with implementation in `tools`. The [early mise discovery policy](../../.config/miserc.toml)
excludes ambient configuration before tool declarations load. CUE, GoReleaser,
Renovate and [ETHOS adoption](../../.ethos/) have distinct consumers: do not add a
second command, hook, lifecycle or dependency authority.

Remove default-only configuration with its consumers. Prettier's locked defaults
and exact Git inventory own formatting; EditorConfig owns editor/byte defaults.
Taplo's intentional deviations preserve array/key order, multiline arrays,
100-column layout and compact inline tables. Neither tools nor upgrades may
silently reorder meaning. [Text policy](text-layout.md) and
[output ownership](../../CONTRIBUTING.md#output-ownership-and-cleanup) own those details.

## Dependency and Framework Admission

Adopt maintained libraries when they reduce total production, test, security,
platform and operating burden. Name the responsibility replaced, demonstrate its
benefit through actual consumers and remove superseded mechanics in the same
Change. Security or portability can justify more code; neither line count nor
feature breadth alone establishes value. Comparisons remain
[research](../research/provider-tooling-assessment.md); irreducible adopted rationale
belongs in the [decision register](../decisions/decision-register.md).

## Dependency Maintenance

[Renovate](../../.config/dependencies/renovate.json5) is the single proposal policy.
Its [native mise manager](https://docs.renovatebot.com/modules/manager/mise/) and Go/npm managers read authored manifests and locks; [`includePaths`](https://docs.renovatebot.com/configuration-options/#includepaths)
excludes disposable snapshots. Declarative extractors read authored Actions/images,
not generated peer workflows. `min_version` is a reader floor, not the executed
Mise pin; the CUE image owns the latter. Dependabot cannot cover this mise graph
and would introduce competing proposals.

OSV uses the official native release with lock-bound asset checksums/SLSA identity.
Its embedded compiler patch is not AIGW's language contract. Do not translate the
authored dependency name or rebuild it solely to align that patch.

Cache the digest-pinned `vars.renovate_image` explicitly, then run:

```bash
mise run dependencies:check
mise run dependencies:inspect
```

These two-minute, disposable operations validate config/extract inventory with
read-only source and no network, credentials, signing agent, Docker socket or HOME
mount. They do not pull, generate proposals or prove freshness. Suppressing a
lookup-token warning is confined to the extractor that performs no lookup.
The host-container recipe has macOS evidence, not implied Windows acceptance.

Use a runnable upstream CI image (`mise-debian`) with a complete immutable
multi-platform reference; do not compensate for scratch images with entrypoint
hacks. Derive its projections from CUE's one version/digest literal. Windows
Runner ownership provides the installed Mise executable; each native job verifies
its exact release/shim hashes, version and identity, then confines config/cache/
state/installs to private job state. Read/copy/remove shim admission and exact
cleanup must succeed under both review and protected identities. Update the shared
binary only in a verified zero-job window.

Peer-local tool copies transport only lock-selected upstream bytes. GitLab's
registry mirrors GitHub assets and required release metadata; anchored metadata
rewriting precedes general URL rewriting and job-local CI_JOB_TOKEN netrc is
deleted with its owned scope. GitHub's `mise-glab-v<version>` tool release holds
original glab filenames; CUE rewrites both locked browser and encoded API URLs
at the install step. This transport release is not an AIGW version or Latest.
The lock remains checksum SSOT; independent SLSA/Sigstore evidence supplies
provenance. Missing or altered copies fail without fallback to the sibling.
Inventory and local mirror tests do not prove hosted authorization, TUF access,
sibling-outage isolation or the complete native cold graph.

Normal proposals wait three days; absent publication times are not guessed.
Group Go and tools separately. Only non-major updates of stable dependencies may
auto-merge; toolchains, Actions, images, pre-1.0 and major versions need deliberate
admission. Security fixes bypass age/quota, not signing or quality. Renovate's
direct OSV integration does not replace full-lock scanning.

`@mermaid-lint/core`'s jsdom 26 range retains deprecated `whatwg-encoding`. The
single jsdom override selects an admitted nondeprecated, fully attestable graph;
its child ranges remain native. Install, signatures/attestations, vulnerabilities,
valid/invalid fixtures and all diagrams must pass. Any deprecated npm package
blocks lock admission. Remove the override when upstream supplies equivalent
native coverage; do not add other overrides to evade authentication failures.

[Renovate's local platform](https://docs.renovatebot.com/modules/platform/local/) only extracts/looks up. Candidate calculation uses the
selected Forge's [`dryRun=full`](https://docs.renovatebot.com/self-hosted-configuration/#dryrun) and native policy, retains proposed contents and
exact base, and performs no publication. Resolver output can be inconsistent:
the lane owner admits exact paths, runs native lock/projection refresh, bootstrap
and provenance checks, then signs through the current Git identity.

Maintenance selects one review peer and distributes the same signed object to
other selected peers through existing publication after verification. Candidate
calculation is not completed signed integration; scheduling needs separate
operator authorization.

Renovate owns an independent disposable clone, never a live lane/writable object
store/signing agent. Its execution owner supplies bounded no-prompt process
credentials without mounted secret files. [`gitPrivateKey`](https://docs.renovatebot.com/self-hosted-configuration/#gitprivatekey) is PGP, not an SSH key
reference. Claiming governed hooks requires operator-owned [`gitNoVerify=[]`](https://docs.renovatebot.com/self-hosted-configuration/#gitnoverify) and
observed rejection/success; repository config cannot override its default bypass.
Observe active leases/proposals before one explicit maintenance run, resume owned
work rather than create competing proposals, and verify exact merge/source deletion.
A scheduler is not required for this owner-driven path.

## Change Lifecycle

[Workspace roles](../../.ethos/workspace.toml) distinguish accepted `dev`, release
`main` and the one authoring Work Lane; `proposal/*` is a review projection, not
another lane. Active intent may remain on accepted/release branches while delivery
is unfinished. Follow [OpenSpec merge then archive](https://github.com/Fission-AI/OpenSpec/blob/v1.13.2/docs/team-workflow.md#when-to-archive):
finish implementation/candidate tasks, archive before stable tagging, then rebuild/
qualify the final signed source. Publication/install/retirement remain post-archive
obligations, not self-referential archive checkboxes. [ETHOS gate](../../.ethos/profile.toml)
and [release](../../.ethos/release.toml) declarations do not replace lifecycle authority.

Current user authorization, repository truth and the exact installed ETHOS result
select authority. One OpenSpec Change owns intent and tasks; Commitment is transient
compilation. Before tracked edits obtain exact-path `lane prewrite`. Keep semantic
closures single-writer, review peers before merge, archive through official OpenSpec,
and re-prove the archived source. A checkbox, branch or file grants no authority.

Package/Provider/client support requires actual artifact journeys, not source-green
or accepted trees alone. Keep the published predecessor's configuration and Adapter
active during replacement. Its reader, schema and native-store identity are distinct
from current fixtures. macOS retained-store tests require disposable host opt-in;
Windows Credential Manager and Linux Secret Service have separate native evidence.

Update and rollback share durable staging and the existing filesystem library's
bounded [rename/remove operations](https://github.com/rogpeppe/go-internal/tree/v1.16.0/robustio). A temporary startup verifier is removed before
replacement; persistent cleanup failure preserves the installation. Never retry the
whole update, delete the current program before the first rename, or add delayed
cleanup services. Two renames provide recoverable replacement, not power-loss
atomicity or uninterrupted visibility. Activation failure restores current bytes;
restoration failure reports both causes and retains recovery material.

Rollback first verifies the actual retained program and its public export against
byte-copied successor configuration in private HOME, environment backend and no
Tokens/client paths. Require a nonempty valid TOML declaration; this proves schema
readability, not inference. Rejection preserves programs/configuration;
`aigw config migrate --rollback` restores exact predecessor state before retry.
Native Windows tests hold real no-delete-sharing handles for update/rollback source,
current and predecessor locks, delayed release and persistent failure. Keep native
errors and surviving file bytes; compilation is not execution.

## Commit and Tag Identity

One commit/tag is constructed and signed locally, then transferred unchanged:
OID, peeled commit/tree, author, committer, annotation and product signature.
[Workspace policy](../../.ethos/workspace.toml) owns the complete subject grammar;
strict SemVer shared with upgrade/construction owns `v` tags and build metadata.
ETHOS hooks and hosted object checks consume that policy, not copied regexes.

CUE selects exact introduced-range verification and independent native matrices for
review, dev/main push and tags on each chosen peer. Manual diagnostics default to
the selected commit's first parent; explicit `commit_base`/`AIGW_COMMIT_BASE` admits
a longer range. Manual or same-SHA sibling checks cannot replace required events.
Review-target-main, accepted-ref parity, tag/version and release observations retain
separate obligations. A green review does not imply merge or proposal deletion.

Same-project GitLab MR admission requires equal source/project IDs and disabled
fork-parent pipelines. This closes a fork path, not arbitrary developer-code trust
or persistent Shell credential isolation. Keep proposals unprotected and provide
separately identified disposable native executors before claiming safe admission.

Transport SSH/PAT/OIDC keys can differ across peers without changing product bytes.
Forge `Verified` is an account projection; its trusted signer principal is not the
Git author/committer. Reject history rewriting/replay, peer-qualified tags, continuity
maps, tree-only/suffix parity and per-peer re-signing.

## Independent Forge Publication

Zero, one or two peers are valid. Local operation remains complete; neither peer
queries, downloads from, repairs or authorizes the other. A failed peer remains
incomplete while independent work continues. [Forge Operations](../operations/forge-operations.md)
owns commands and exact-effect observations.

Publication metadata/upload requests use the explicit origin without redirects.
Downloads retain native CDN policy/bounds: changed scheme/host/effective port removes
credentials for the entire remaining chain, even if it returns; HTTPS never downgrades.
GitHub's separate upload target must be an absolute HTTP(S) URL without userinfo and
preserve HTTPS. Direct asset credentials stay at the selected API authority.
These rules belong to publication, not changed global HTTP clients or retries.

`main` atomically updates peer main/dev to the same object; `proposal/*` updates
only its matching ref. Work/candidate/dev/arbitrary refs are not publication inputs.
Every write carries an exact observed lease, including expected absence; even explicit
fast-forward/equal-tip expectations must match. Divergence needs bounded destructive
authorization and immediate restoration of protected force-push controls. Formal tags
remain immutable unless separately proved failed/intermediate and authorized for
retirement; age alone is not deletion authority.

## Release Chronicle

[CHANGELOG](../../CHANGELOG.md) starts with `## [Unreleased]` for post-release changes.
Version headings are unique descending strict SemVer; only the first may be pending
and equal [VERSION](../../VERSION). Older headings require exact signed tags/dates.
Build metadata belongs to identity, not precedence; metadata-only duplicate entries
cannot establish descent. Epoch lookup keeps complete version text and rejects numeric
overflow. A tag proves source identity, not assets, installation or Apple approval.

## Reproducible Assets

Exact compiler, Go/npm closure, standalone tools and sidecars come from their tracked
locks; release epoch comes from committed chronology. Untagged candidates instead bind
current version and exact source epoch, never invented dates. No tag/publication or
installation is implied by construction.

Credential-free GoReleaser applies deterministic ad-hoc Mach-O/Hardened Runtime
signatures before archives; native codesign validates extracted bytes. This is local
integrity, not publisher/Gatekeeper proof. Native tests clear inherited signing identity.
Operator Developer ID builds require an explicit existing certificate SHA-1 identity
on macOS; the hook applies Hardened Runtime and trusted timestamp before packaging,
without exported keys, imported trust or access-policy changes.

Reproducible CI signatures use release time; timestamped Developer ID bytes are a
separate distribution artifact, not a repeatable-byte claim. Sign once, verify final
archives and publish the same matrix to both peers. After Apple accepts, use:

```bash
mise exec --locked -- go run ./tools/release verify-macos-distribution \
  "$ARTIFACT_DIRECTORY" "$VERSION" "$AIGW_MACOS_SIGNING_IDENTITY" \
  "$AIGW_MACOS_NOTARY_ARCHIVE" "$AIGW_MACOS_NOTARY_SUBMISSION" \
  keychain-profile "$AIGW_MACOS_NOTARY_PROFILE"
```

The verifier binds archive checksums, exact certificate, Apple submission/log, uploaded
ZIP checksum and both executable members. Use one explicit profile or protected API
key mode; Team keys need issuer ID, Individual keys omit it. Do not copy secrets to
arguments/source or silently switch modes. All stable publication entrypoints require
this verification before Forge access. `spctl --type execute` is not standalone-CLI
admission; valid notarized code can fail that App-oriented assessment. Portable binaries
cannot carry stapled tickets: fresh Gatekeeper discovery remains online. Follow the
[Apple procedure](../operations/forge-operations.md#macos-signing-and-notarization).
API-key mode uses `AIGW_MACOS_NOTARY_API_KEY_FILE`, `AIGW_MACOS_NOTARY_API_KEY_ID`
and Team-only `AIGW_MACOS_NOTARY_API_ISSUER_ID`; profile mode uses
`AIGW_MACOS_NOTARY_PROFILE`. Native store tests additionally require
`AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE=ephemeral-host` on macOS.
RC publication/download verification remain portable without publisher credentials.

Detached `aigw-release` signatures independently authenticate complete checksums.
Keep source-bound provenance, SBOM, vulnerabilities/licenses and exact peer parity.
Retained Token authorization has its own [credential decision](../decisions/dr-0011-single-portable-token-backend.md#release-identity-and-credential-authorization).

Syft catalogs every native binary, including platform dependencies, through the explicit
[policy](../../.config/release/syft.yaml), not ambient user config, module caches, remote
lookups or vendor enrichment. Tests bind every reported path/digest to the generated
matrix. Normalize paths through a root-confined handle; actual SHA-256 integrity and
[SPDX-required SHA-1 metadata](https://spdx.github.io/spdx-spec/v2.3/file-information/#84-file-checksum-field) are computed, not guessed. Either path separator/one leading
root separator is accepted; traversal, network paths and duplicate normalized names
fail. This addresses [Syft's directory-digest gap](https://github.com/anchore/syft/issues/4564)
without weakening scanner or Windows coverage. SBOM runtime discovery does not replace
full-lock licenses/vulnerabilities.

OSV invocation and admission share exact selected lock paths. Require each once as a
lockfile source with observed packages; empty/partial reports, another checkout or
ordinary directory scans are not clean evidence. Strip host prefixes only after this
check, before signing/replacing output. Each peer publishes the same immutable files;
its assets never become another peer's build input.

### Homebrew packaging projection

[GoReleaser](../../.config/release/goreleaser.yaml) retains generated
`dist/homebrew/Casks/aigw.rb` separately from signed assets. Windows-only builds
skip it; Linux/macOS use the same native generator. Generation creates neither tap
nor release. Verify archive-internal paths/checksums and regenerate after signing;
ad-hoc fixture bytes cannot qualify notarized Cask output.

The generated Cask is a derived artifact from GoReleaser, not another editable release
owner. Publish it only after source/archive/Apple acceptance and exact dual-peer assets.
Homebrew owns its installation and public link; portable self-update cannot overwrite
that owner. Installed hot upgrade, retained readers, rollback and exact old-byte
retirement require their own package-manager journey. [Team rollout](../guides/team-rollout.md#release-installation)
explains member installation.

## Quality and Platform Evidence

### Scope and authority

One [quality graph](../../tools/ci/quality.go) maps every product/tool/test/config/doc
carrier to required concerns; native configs and executors remain below it. Formatting,
Go vet/lint/types, race/coverage, SCC, schemas, Markdown/Mermaid/links, secrets and
artifact/runtime checks each have one owner. Architecture classification is not check
effectiveness; add every applicable concern without another registry or generic parser.

[Text policy](text-layout.md) owns exact inventory, Prettier, markdownlint, Mermaid,
Taplo and typos. Actionlint invokes locked ShellCheck for embedded workflow fragments;
GitLab relies on CUE projection and actual native execution. No separate YAML formatter,
Pants/Dagger/Nix/CEL plane is admitted without demonstrated replaced responsibility and
native three-platform benefit. OpenSpec INFO remains visible advice; WARNING/ERROR,
unknown severities, malformed reports and failed summaries block. Cohesive requirement
length prompts semantic review rather than arbitrary fragmentation.

[ETHOS gates](../../.ethos/profile.toml) select execution, not sandboxing: prepared
behavior fixtures are offline; signature/OSV quality needs network evidence.
Warm cache is neither offline proof nor independent cold acquisition.

### Native platform evidence

Support needs native macOS/Linux/Windows evidence across the declared aggregate set.
An unavailable executor remains unproved, not an indefinitely pending/allowed-failure
substitute. Cross-compilation only constructs targets. Run exact portable archive
install/update/rollback/uninstall with retained state and native credentials separately.
Signature, code signing, notarization, client inference and source acceptance are
independent; a Forge's complete required source matrix still needs its own runners.

Every native job executes shared Go static policy before behavior/lifecycle, with no
parallel vet wrapper. Windows ABI annotations stay on necessary syscalls; pinned Job
information, copied-before-free DPAPI buffers and metadata-only credential observation
retain their actual security boundaries. A value-reading library cannot replace
metadata inspection merely to shorten code.

Release acquisition retains its selected caller's transport authorization.
Construction and acceptance share one environment override owner that clears Forge
tokens, job-netrc pointers and credential commands, and disables Mise credential
fallbacks. This does not isolate the filesystem: untrusted Runner code still needs
separate native identity, storage and execution containment.

### Behavioral and quantitative evidence

[Coverage policy](../../.config/checks/coverage/policy.toml) owns strictly greater-than
95% native Go statement coverage across every product/tool/tested platform package.
No package/source exclusion or zero-denominator percentage is permitted. Observe
complete packages, use actual native numerator/denominator, and keep rounded display
separate from comparison. Unit count and branch inference cannot replace the metric.

Every measurable package must execute owned statements; declaration-only packages
need source evidence and remain not-applicable, never 0%/100%. Package ratios inform
review, not another independent veto. Instrumentation/observation share exact sorted
Go-selected inventory; unexpected counters reject rather than change denominator.
Retain raw counts, package identities, revision/tree/toolchain/policy digest and never
relabel statement as branch coverage.

[Go policy](../../.config/checks/go/policy.yml) and [SCC policy](../../.config/checks/go/size.toml)
are the only editable thresholds. Their measurements have distinct meanings:

| Analyzer         | Measurement and limitation                                                                             |
| ---------------- | ------------------------------------------------------------------------------------------------------ |
| SCC              | Code lines including inline-comment code; excludes blank/comment-only lines, not executable statements |
| Cyclop           | Boolean/switch decision score, not executable paths or adequacy                                        |
| Gocognit         | Nested control flow including function literals, not deep-module quality                               |
| Funlen           | Physical span and selected AST statement forms; callbacks/traversal have different coverage            |
| Revive arguments | Named declaration parameters; receivers/unnamed parameters differ                                      |
| Nestif           | Conditional reasoning score, not indentation depth or every loop/switch                                |
| Maintidx         | Span/vocabulary/decisions combined; cohesive data can score poorly                                     |
| Dupl             | Serialized syntax-tree similarity, not repeated responsibility                                         |

Complete transactions retain ordered checks/compensation and acceptance journeys retain
all intermediate preservation assertions. Do not evade thresholds with forwarding
helpers, unrelated parameter bags, dense literals, exclusions or softer test policy.
Similar native operations are not automatically duplicate responsibility.

#### Calibration decision

Trials covered macOS arm64, Linux amd64 and Windows amd64 product/tests/tools. Retained
limits are a reasoned trade-off, not a universal optimum or test-volume claim:

| Trial                  | Decision and reason                                                                                                             |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| Cyclop 20 then 15      | Keep 25: atomic manifest/ACL admission and complete lifecycle assertions do not justify blanket forwarding extraction           |
| Cognitive 40           | Keep 45: bootstrap/recovery/native acceptance combine ownership and cleanup                                                     |
| Span 110/statements 55 | Keep 120/60: explicit input/data and complete journeys remain inspectable                                                       |
| Arguments 6            | Keep 7: Account/Route and before/after artifacts are real distinct inputs; bags alone remove no knowledge                       |
| Nestif below 4         | Adopt: early terminal paths and shared metadata deletion remove real nesting; positive 3/failing 4 fixtures cover product/tests |
| Maintidx 30            | Keep 25: vocabulary/data cost remains bounded by independent size/nesting/decisions                                             |
| Clone 80               | Keep 100 tokens: DPAPI protect/unprotect and distinct assertions are separate operations                                        |

Machine values remain solely in native policy. The October 1 Cyclop recalibration
is closed: retain 25 across product, tools and tests, not as a universal optimum.
Trials and representative ownership review are recorded in the active Change.
Findings alone are not defects or a reason to add shallow helpers or an assertion
dependency. Reopen for a concrete escaped risk or semantic redesign, not another
unchanged trial. Native tests retain all applicable diagnostics; suppressing
companion rules cannot count as calibration.

### Check effectiveness and failure semantics

Positive/negative boundary pairs run real check owners across product/tests/foreign
platform selections with bytes preserved. Removing the intended rule must invalidate
its expected rejection. Configuration schema, complete inventory, output failure and
warnings are tested through their owners; self-green cannot prove hosted admission,
visual rendering, live Provider or installation.

Correctness includes vet nilness/unused-write and checked dynamic assertions; concrete
fixtures should not recover known types from broad interfaces. Discarded errors need
owner-specific justification. Progress output must be writable before executing gates.
A failed report after successful publication reports both facts rather than retries or
rolls back the external effect. Read-only close and infallible memory operations differ
from durable-write failures. Redacted security copies preserve policy/path scope, not
symlink targets or implicit history scans.

### Performance and completion claims

The current release review's engineering budgets are scoped to equivalent native
inputs, not guarantees for every host:

| Boundary                                   | Budget or review threshold                                    |
| ------------------------------------------ | ------------------------------------------------------------- |
| Warm version/help/configured status/export | p95 at most 100 ms                                            |
| Projected helper including native shell    | p95 at most 100 ms                                            |
| Binding change, first setup, or sync       | p95 at most 250 ms                                            |
| Peak resident memory                       | Review growth exceeding both 20% and 4 MiB versus predecessor |
| Same-target uncompressed executable        | Review growth exceeding both 10% and 1 MiB versus predecessor |

An exceedance needs repair or an explicitly accepted measured product trade-off;
never silently waive it. Exclude inference/client startup/human authorization from
local budgets but report them separately, not subtract them from end-to-end results.
Each setup sample starts without the owned AIGW configuration or managed client
projection; a bounded test-binary preparation resets only those fixture files
outside the timed command. Credentials and the installed program remain intact.
Sync starts from a converged explicit binding. Both include guarded durable
projection work with warm process and filesystem caches; human authorization,
cold-cache onboarding, repository construction and CI duration remain separate.
Use five warmups and two reversed-order blocks of at least forty samples; retain
outliers, per-block and pooled p95. Host-contention/order sensitivity means inconclusive,
not a raised threshold. Backend/OS/client/tool identity remains explicit.
Any native benchmark warning makes the acceptance command fail after preserving
all samples and the summary, even when every measured duration is within budget.

Use native [candidate performance](../operations/forge-operations.md#measure-native-candidate-performance)
with one exact signed matrix and retained predecessor on a quiet host. Preserve raw
samples, reversed blocks, warmups, outliers and native peak-memory calibration. Timed
synthetic endpoints prove overhead, not Provider latency or real-client inference.
Any exceeded budget requires measured repair or an explicitly accepted product
trade-off; rerunning identical failed inputs or silently raising a limit is not acceptance.

Archive changes source identity; final bytes/provenance and exact-HEAD governance need
fresh admission. Reuse unchanged immutable observations only for the claim they prove.
Source, checks, signed objects, hosted CI, artifact hashes, Apple acceptance, installation,
reader continuity and cleanup are separate facts. Complete delivery requires current
observations of each selected surface, not a checkbox or proposed operation.

## Branch and Worktree Closeout

Merge only the admitted object through current ETHOS/peer policy. Verify exact target
OID, review state and source-ref deletion on each peer; null GitLab merge_commit_sha
can be a successful fast-forward. Retire the exact owned lane through ETHOS after
pending evidence and active consumers are preserved. Do not delete foreign/unknown
work or obsolete bytes merely by age/name. [Forge closeout](../operations/forge-operations.md#observe-publication-and-integration)
owns the reproducible checks; [output ownership](../../CONTRIBUTING.md#output-ownership-and-cleanup)
owns cleanup targets.

## Product Boundary

AIGW never carries traffic, owns sessions or manages external services. IDE state,
Codex JSONL/SQLite, historical messages and per-conversation model metadata remain
client-owned. Proxy activation
is optional and operator-owned; explicit endpoints remain ordinary Account data.
Require real consumer evidence before claiming any Adapter or compatible protocol.
