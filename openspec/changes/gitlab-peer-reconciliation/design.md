# Design

## Owners

ETHOS owns branch and tag publication to each peer. Publish GitLab `dev` and
`main` with `ethos publish --ref <ref> --peer gitlab --probe-remote
--expect-head <head>` and apply its receipt; both refs fast-forward, so no merge
request is needed. ETHOS `e05c372cf` lets a peer that missed a release receive
its attested tag when the release commit is in accepted history, so each tag
keeps the object GitHub received. The maintenance tags `v0.3.4` and `v0.3.5`
were created with `--release-base` from a maintenance line outside accepted
history. Their catch-up needs the maintenance publication admission on ETHOS
lane `work/20261002-native-trust-closure`, delivered through accepted ETHOS
`dev` and an aigw-cli runtime rebind. ETHOS's repository paradigm admits only
catalogued root entries, so that rebind first needs the root `.cbmignore`
removed.

The GitLab pipeline selects peer tool supply. Its mirror package is versioned by
the exact SHA-256 of `.config/mise/mise.lock` and must hold verified upstream
assets; AIGW does not produce it. The operator decides whether to publish that
package or to run GitLab with upstream supply where its runners reach GitHub.
Pushing branches does not repair supply. On 2026-10-14 the operator chose
upstream supply: the project's `AIGW_TOOL_SOURCE=peer` override was removed, so
the CUE-projected default applies. With that supply, GitLab's protected `dev`
and `main` pipelines for `17d9bdf6` passed on 2026-10-16 (11252 and 11254).

Peer admission reuses the checks that `terminal-product-convergence` task 7.4
applied to GitHub. The ETHOS identity and reference successor is ETHOS's own
deliverable; this Change consumes the accepted release and adds no private
checker.

## Maintenance tags

On 2026-10-15 the user authorized a break glass for `v0.3.4` and `v0.3.5`:
their existing signed tag objects, equal to GitHub's and passing
`git verify-tag`, were pushed to GitLab with `git push --no-verify`. GitLab
reads back `e86777be` and `95132368`. ETHOS has no publication receipt for
them; task 3.1 reconciles that history.

## Releases

Each peer's Release comes from the local release bundle, never from the other
peer. GitLab received `v0.4.1` on 2026-10-16 from its original bundle. `v0.4.2`
stays without a GitLab Release: its only bundle was retired with its lane, and
the user decided on 2026-10-16 to leave it absent rather than copy GitHub's
assets. The `v0.3.4` and `v0.3.5` bundles remained, but not their original
notarization uploads, which publication verifies. At the user's direction their
unchanged executables were notarized again (Apple submissions `a1f04a4c` and
`7520ec6a`), and on 2026-10-17 both Releases were published from the retained
bundles with 12 asset links each.

## Admission

Task 2.1 read GitLab's effective project policy, protected refs and runners
on 2026-10-16:

- Merge requests merge only by fast-forward, after a passing pipeline and with
  resolved discussions. `dev` and `main` accept pushes and merges from
  Maintainers only and refuse force pushes; protected `v*` tags can be created
  only by Maintainers.
- `proposal/*` branches stay unprotected. Review pipelines run only for
  same-project merge requests, and fork pipelines are disabled.
- Every native and quality job reads the public signer anchors from GitLab's
  own project variables, `AIGW_RELEASE_ALLOWED_SIGNERS` and
  `AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS`.
- Runners are project-scoped, locked and tag-only. Review and protected
  execution use separate `not_protected` and `ref_protected` registrations
  with separated caches; shared and group runners are off.
- Linux review jobs run in disposable containers. Windows review isolation
  stays a bounded limitation, because the protected-file probe needs a Unix
  host.
- `remove_source_branch_after_merge` is on, but a merge recorded by ETHOS's
  fast-forward push deletes nothing, so ETHOS retires proposal refs itself.

The macOS review runner runs as `ci-aigw-review`. The protected runner's
configuration is a root-owned `0600` file inside two `0700` directories, so the
first probe on 2026-10-17 (job 54193) failed: the review identity could not
even `stat` the file, and the probe stopped before its elevation check. The
probe now counts a denied traversal as a denied read, still requires
`sudo -n -l` to show no grant, and reports the resource identity as
operator-asserted, because the review identity cannot observe it. Relaxing the
directories to fit the probe would weaken the isolation it measures.

## Order

Catch-up precedes admission: GitLab must first hold the accepted branches,
tags and tool closure before its protected checks can describe them. The ETHOS
successor is independent and may complete in either order.
