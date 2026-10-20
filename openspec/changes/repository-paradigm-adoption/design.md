# Design

## Context

ETHOS validates every adopter against one repository paradigm. AIGW's bound
runtime predates the paradigm gates, while a runtime built from current ETHOS
`dev` reports 96 gaps on the accepted head and refuses every proof. The CI
graph lives in `.config/ci/pipeline.cue`, which projects GitHub and GitLab
files. Its GitLab native jobs assemble acceptance arguments in three shell
programs (bash on Linux and macOS, PowerShell on Windows) and prepare Mise
state with job-owned shell before any locked tool exists.

## Goals / Non-Goals

**Goals:** zero paradigm and decision-record gaps on the lane head under the
new runtime; unchanged native acceptance semantics for every declared input;
the rebind in the same batch.

**Non-Goals:** changing the peer Mise mirror layout, the runner images, the
release flow, or which jobs run for which events.

## Decisions

### One Go owner for native job inputs

`tools/ci native --from-env` reads the GitLab inputs that the shell programs
read and produces the same acceptance arguments, guard and environment. It
replaces the bash and PowerShell programs with one tested implementation. The
Linux Secret Service session becomes a re-execution of the same command under
`dbus-run-session`, with the alias set through `gdbus`. The Windows public
input supply downloads and verifies the package in Go and extracts its client
supplier with the system `tar`. The source bootstrap and a requested lock
refresh run before source qualification in the same command, so an
artifact-only run still skips both. Each branch of the former shell is a table
case. GitHub's Windows client supply and its cleanup become
`tools/ci native-clients`, and release evidence resolves the tag commit itself.

### Forge conditions select the artifact tool scope

The shell narrowed `MISE_ENABLE_TOOLS` for artifact-only runs and skipped the
Linux compiler there, which the native artifact closure requirement keeps.
GitLab now selects those variables with `rules:variables` from the same inputs,
as GitHub already selects them with workflow expressions. Each admission rule
of a native job is preceded by its artifact-only scopes, so the selection holds
in every pipeline that admits the job.

### Shell only where no locked tool can run

Some steps run before `mise install` provides Go, or in `after_script` after
it may have failed: apt packages, the D-Bus session wrappers, the job-owned
Mise state and download source, its cleanup, and Windows' verification of the
runner-owned Mise. They stay shell and are recorded by Forge file, jobs and
text digest. Any edit makes them inline shell again until re-recorded.

### Declarations defer to the shared paradigm

The workspace table only restated the shared release-mirror role and the SSH
signing policy, so it is removed. The release mirror moves to the release
declaration. AIGW's commit-provenance verifier keeps verifying SSH signatures
from the shared policy and refuses a table that restates them.

### Rebind in the batch

The runtime switch uses the runtime's own `hook install` after the migration
commits exist in the lane. The proof of the lane head then runs under the new
runtime. The previous runtime's own CLI rolls the binding back if the batch
cannot land.

## Risks / Trade-offs

- [Manual input paths run only in manual pipelines] → table tests reproduce
  every branch of the former shell, and the default pipelines on both peers
  exercise the empty-input path on all platforms.
- [GitLab evaluates the artifact-scope rules only when manual inputs are set]
  → projection tests pin each scope's condition and variables, and every
  default pipeline validates the rule syntax.
- [A runner image gains a tool and makes a retained step unnecessary] → the
  record names its owner and reason, so review can retire it.
- [The new runtime tightens rules again before landing] → the batch rechecks
  the gaps with the runtime it binds.

## Migration Plan

1. Land the batch under the new runtime after a full proof.
2. Publish through the review and publication paths on both peers and confirm
   the pipelines on all three platforms.
3. Under `gitlab-peer-reconciliation`, preview and apply the `v0.3.4` and
   `v0.3.5` receipts with the new runtime.

Rollback: rebind with the previous runtime's own CLI and revert the batch.

## Open Questions

- None.
