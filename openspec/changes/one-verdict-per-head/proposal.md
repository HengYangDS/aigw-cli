## Why

The task rules in `openspec/config.yaml` ask every Change to "separate local gates, exact-HEAD proof, hosted CI,
publication, and housekeeping", so each Change records its proof, its hosted runs and its proposal publication as tasks.
Every task must be complete before the archive, and the archive commit is a new head with its own proof and, on GitHub,
its own required checks. So each Change is proven twice and verified twice on each forge, and an unavailable peer stops
its archive.

The accepted requirement "Terminal local release readiness" of `release-distribution` already says the opposite: Change
tasks close implementation and candidate acceptance before the archive, hosted CI and delivery consume the accepted
result instead of gating it, and the archive neither claims those outcomes nor blocks local acceptance when a peer is
unavailable (scenario "External delivery follows local readiness"). On 2026-11-28 `codex-probe-once` showed the cost:
18 of about 33 minutes of its heavy local work repeated work already done, GitHub ran its checks again at the archive
head, and a GitLab that had stopped creating pipelines held its archive until the task moved to another Change.

## What Changes

- The task rules state that a Change's tasks hold its implementation and the local gates that close before the
  archive, and that the exact-HEAD proof of the archive head, hosted CI and publication are landing gates that ETHOS
  and the forges enforce and record, never tasks.
- The rule that separated those kinds of task goes; the rule that a Change never records its own landing, archive or
  lane retirement stays.

Not breaking: the product, its commands, files and outputs do not change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change changes no behavior of the product, so it declares `skip_specs: true`; the accepted requirement
"Terminal local release readiness" of `release-distribution`, scenario "External delivery follows local readiness",
already states the behavior that the rules now follow.

## Impact

`openspec/config.yaml`, which the OpenSpec instructions give to every author of a Change. Authority boundary: ETHOS
keeps the proof and landing, each forge keeps its CI records and branch protection; nothing in AIGW records their
results a second time. Reuse: no new check or tool; GitHub's protection of `dev` still requires its checks on the exact
published head. Non-goals: native acceptance as a proof gate, which needs its own measurement first; reuse of one
forge's verification across events of the same commit, which ETHOS takes up for every repository.
