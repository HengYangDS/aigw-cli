---
subject: aigw:dependency-maintenance
role: policy
state: canonical
relations:
  canonical_for: dependency maintenance and release age
---

# Dependency Maintenance

[Renovate](../../.config/checks/renovate/renovate.json5) is the single proposal policy.
Its [native mise manager](https://docs.renovatebot.com/modules/manager/mise/) and Go/npm managers read authored manifests and locks; [`includePaths`](https://docs.renovatebot.com/configuration-options/#includepaths)
excludes disposable snapshots. Declarative extractors read authored Actions,
images and exact npm client pins in CUE, not generated peer workflows. Native
client proposals keep deliberate admission.
The repository's release age is declared once, in whole hours, as
`minimum_release_age` in the [mise configuration](../../.config/mise/config.toml);
`mise run fix` fills Renovate's root `minimumReleaseAge` and npm's
`min-release-age` from it, and the `release-age` quality gate fails while either
holds another age or another release rule of its own. One edited line changes the
age for every source. Only a security fix skips the age. With an age above zero,
a registry that publishes no release time leaves its release held until the
publisher's release record dates it.
`min_version` is a reader floor, not the executed
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
The host-container recipe has macOS evidence.

Use a runnable upstream CI image (`mise-debian`) with a complete immutable
multi-platform reference; do not compensate for scratch images with entrypoint
hacks. Derive its projections from CUE's one version/digest literal. On GitLab's
macOS runner, runner ownership provides AIGW's own Mise executable in a directory
named by its release; each job verifies its digest and version and makes the
word `mise` run it. Every GitLab job points Mise's global and system
configuration at the checkout's `.config/mise/external`; only with
`AIGW_TOOL_SOURCE=peer` do Mise's data and cache move to a job-owned directory,
whose exact cleanup must succeed under both review and protected identities. The
host holds a new release beside the current one before the declaration moves
to it, and removes the earlier release once no job runs its declaration.

Tool acquisition defaults to locked official upstreams. On GitLab only,
`AIGW_TOOL_SOURCE=peer` selects that registry's checksum-verified copies; it is
not an automatic fallback. GitHub jobs always download from the official
upstream that the lock declares.

GitLab's registry mirrors GitHub assets and release metadata using job-local
CI_JOB_TOKEN netrc, removed with its owned scope. CUE projects the selector and
its browser/API rewriting; the lock remains version/checksum SSOT. Missing or
altered selected copies fail.

Native upstream bootstrap and peer-isolated cold acquisition are separate
claims; local inventory cannot establish either hosted result.

Normal proposals wait the declared release age; absent publication times are not guessed.
Group Go and tools separately. Only non-major updates of stable dependencies may
auto-merge; toolchains, Actions, images, pre-1.0 and major versions need deliberate
admission. Security fixes bypass age/quota, not signing or quality. Renovate's
direct OSV integration does not replace full-lock scanning.

Any deprecated npm package blocks lock admission. Do not add npm overrides to
evade authentication failures.

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
