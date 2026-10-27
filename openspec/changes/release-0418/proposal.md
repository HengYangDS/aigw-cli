## Why

AIGW 0.4.18 closes the obligations that keep the maintenance branch and four
open findings alive:

- `proposal/maintenance-client-qualification` (tip `f3c7f87f`, tag `v0.3.5`)
  holds 12 commits whose obligations `dev` lacks (audit of 2026-10-25). The
  user ruled that the branch is deleted only after they are absorbed, and
  decided three of them: an Account edit under forwarding is refused until
  `--direct`; only product diagnostics fail benchmark acceptance; a
  source-built performance candidate stays refused.
- Installed 0.4.17 sends an outside edit to managed Codex settings into a loop:
  previews advise `aigw check`, `check` advises `aigw sync`, and that sync
  refuses on the same state (Codex finding `3d9ee726`).
- On Windows, `aigw doctor` does not say "(outside the home directory)" for a
  Codex target on another drive or share (Codex finding `61c494f1`).
- A GitHub retained-store Verify run with `tool_source=peer` failed before any
  test: artifact jobs prepare glab, and the GitHub carrier lacks the locked
  glab release (BLOCK `f86f8c0d`).
- `TestForgeToolchainsIgnoreForeignMiseConfiguration` records its temporary
  checkouts in the operator's real Mise state.
- The 2026-10-27 catalogue review found newer vendor models: GPT-6.1 Terra,
  Kimi K3.1 Pro, Muse Wonder, Mistral Large 4 and Command A+.

## What Changes

- A forwarded Client Binding records the upstream identity and protocol it
  reaches. An Account endpoint or protocol edit, an Account rename or a
  manifest import that would change them is refused until
  `aigw use --direct`; the destination is never silently rebound.
- A refused outside edit to managed Codex settings carries one next action
  through errors, `check`, `status` and `doctor`: undo the outside edit, then
  run `aigw sync`.
- Doctor reports a target on another Windows volume as outside the home
  directory.
- Benchmark acceptance fails on diagnostics of the product's own prepare and
  measured commands; Hyperfine's statistical warnings stay as evidence.
- Release tooling ignores non-`v` tags when it selects a release, a manual
  tag-ref dispatch honors `native_platform`, an OSV exit 1 without a
  normalized advisory blocks, and the Hermes release journey disables lazy
  installs.
- Artifact jobs prepare only their peer's download tool: gh for GitHub, glab
  for GitLab.
- The Mise configuration test keeps Mise state inside its own workspace.
- The team catalogue admits the newer vendor models whose Routes qualify.
- The branch's remaining regression tests are ported where `dev` already has
  the code.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `route-client-selection`: forwarding keeps the upstream it records.
- `product-control-plane`: an outside edit to managed Codex settings has one
  recovery action.
- `product-quality`: benchmark diagnostics, dependency scan exits, doctor
  volumes, artifact tool closures and private Mise state in tests.
- `ci-diagnostics`: release tag selection and manual tag-ref dispatch.

## Impact

User-visible: the forwarding refusal, the Codex recovery action, the doctor
label on Windows and the team catalogue. Release and CI tooling, both generated
Forge projections and tests also change. Configuration and checkpoint formats
stay compatible with retained 0.4.17 readers. The maintenance branch ref is
deleted after acceptance; its tags `v0.3.5` and `native-inputs-f3c7f87f…` stay.
