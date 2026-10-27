# Design

## Context

The maintenance branch qualified 0.3.x clients and tooling after `dev` moved
on, so some of its behavior and many of its tests never reached `dev`. The
audit at `build/evidence/maintenance-absorption-audit-20261025/audit.md`
lists them; this Change rechecked each against `dev` `ed9f0fd5`. Four later
findings and one catalogue review join it so that one release closes them.

## Goals / Non-Goals

**Goals:** absorb every branch obligation that `dev` still needs, make each
refusal give one action that can succeed, and keep retained 0.4.17 readers
working.

**Non-Goals:** new commands, a new tool source, a Windows process inventory,
changing the coverage floor, or ranking models beyond vendor statements and
Route qualification.

## Decisions

### Forwarding records the upstream it reaches

A forwarded Client Binding keeps the upstream identity and protocol that its
sidecar already stores. Encoding writes them back unchanged, and every write
path compares the bound Route's current Account with them: a mismatch from
`account edit`, an Account rename or a manifest import is refused before any
write, with `aigw use --for <client> <route> --direct` as the next action
(plus `--native-retries` when a retry policy is selected, so the advised
command passes its own guard). Selecting the same upstream keeps forwarding;
`--direct` withdraws it completely. The checkpoint JSON keeps its shape,
because a retained 0.4.17 reader decodes it strictly; restoring a checkpoint
binds each forwarded binding to its own recorded upstream.

Rejected: rebinding silently to the edited Account, which is `dev`'s current
behavior and sends traffic somewhere the operator did not choose.

### One recovery action for an outside edit to managed Codex settings

The guard that refuses an outside edit already exists; its errors are untyped,
so presentation falls back to `aigw check` and inspection reports drift with
`aigw sync`. One typed conflict error carries the action "Undo the outside
edit to the AIGW-managed Codex settings, then run `aigw sync`", and errors,
`check`, `status` and `doctor` all show it. Validation checks recorded
ownership before comparing with the desired block, so a conflict is reported
as a conflict. Doctor and verification stop wrapping every action in
``run `…` ``, so a sentence-form action is shown as written.

Rejected: a forced overwrite, relaxing the guard, inferring retry policy, or
advising a command that the guard refuses on the same state.

### Doctor and distinct volumes

When `filepath.Rel` fails, a target is outside the home directory only when
both paths are absolute and their volume names differ, compared without case
as `Rel` does. A relative or invalid HOME stays unknown. The check is
portable; `VolumeName` is empty off Windows.

### Benchmark diagnostics come from the product

An untimed preflight runs the prepare and measured commands once and judges
their diagnostics, as the Windows path already does. Timed Hyperfine runs keep
their raw output as evidence and no longer fail on Hyperfine's own statistical
warnings. No warning parser is added.

### Release tooling

- Release selection returns a tag only when it starts with `v`.
- Native jobs drop the `github.ref_type == 'tag'` clause, so a manual dispatch
  from a tag ref honors `native_platform`. The branch's tag-provenance guard
  is not ported: only `v*` tags start push runs, `dev` creates no transport
  tags, and a dispatch on `native-inputs-f3c7f87f…` runs that tag's own
  workflow, which keeps the guard.
- An OSV exit 1 with no normalized advisory fails the dependency gate.
- The Hermes release journey sets `HERMES_DISABLE_LAZY_INSTALLS=1`, as
  `internal/client` already does.

### Artifact tool closures follow the peer

The artifact tool list is indexed by peer: GitHub prepares gh and GitLab
prepares glab, each with Go and GoReleaser, plus the purpose-specific tools.
Source and full-quality jobs keep both CLIs. A GitHub-only job therefore never
asks a carrier for glab.

### Tests keep Mise state private

The Mise configuration test sets `MISE_STATE_DIR` inside its workspace and
keeps HOME and the data directory, so tools are not reinstalled and its trust
and tracking records never reach the operator's state.

### Team catalogue refresh

A candidate replaces its vendor's Model, or for OpenAI and Anthropic joins the
curated series, only when the vendor documents it as available and each listed
Route passes bounded inference and the native Hermes file-tool and recall
journey. Mistral Large 4 is a Public Preview; it is admitted on the same terms
as the Preview Models already shipped, and its label says so. Candidates whose
Routes fail stay out, with the failure recorded.

### Not ported

- A published baseline with a source-built performance candidate stays refused:
  superseded by decision.
- The branch's Codex-only forwarding restriction: `dev` forwards Hermes and
  Claude too.
- The branch's "`--json` requires `--dry-run`" refusal: `dev` supports
  `aigw use --json`.
- The branch's "restore direct mode before predecessor rollback" scenario:
  0.4.17 reads forwarding, and rollback while forwarded is already tested.

## Risks / Trade-offs

- **Operators who edited a forwarded Account now see a refusal.** That is the
  intended behavior; the refusal names the command that resolves it.
- **The Windows coverage margin is thin.** Windows-only branches stay in
  build-constrained files or are reached by portable tests, and the hosted
  Windows job remains the authority.
- **Preview models can change.** Mistral Large 4 is labeled as a Preview and
  is not a recommendation.

## Migration Plan

Release 0.4.18 through the existing GitHub release path. After the installed
successor passes acceptance, delete the maintenance branch ref through the
GitHub API and locally with plain Git, and keep both tags. GitLab follows its
catch-up.

## Open Questions

- None.
