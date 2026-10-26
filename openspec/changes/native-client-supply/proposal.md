## Why

The release owner declares the native client journeys each host runs: Claude,
Codex and Hermes on Linux and macOS, and Claude and Codex on Windows. Real-client
acceptance requires an explicit executable for each, plus Hermes' installation
home, before it downloads or runs anything. Only GitHub's Windows job supplies
clients. Its supply knows the Windows package layouts and Git Bash, and the
pipeline model gives Linux and macOS no client supply and no client tool
closure. Linux and macOS jobs therefore cannot run the retained-predecessor
journey with real clients, so that acceptance on those hosts is unproved
(BLOCK `82f2a380`).

## What Changes

- `ci native-clients supply` serves the job's own host. It installs exactly the
  clients that the release owner declares for that platform, at the official
  releases that the pipeline model pins once:
  - Codex and Claude Code come from their official npm platform packages on
    every host.
  - Hermes comes from its official release on Linux and macOS. The release's
    own installer runs only after its pinned checksum matches. The result must
    be the unmodified pinned checkout and its published launcher.
- Each supply exports the explicit acceptance inputs for its host:
  `AIGW_ACCEPTANCE_CODEX`, `AIGW_ACCEPTANCE_CLAUDE`, `AIGW_ACCEPTANCE_HERMES`,
  `AIGW_ACCEPTANCE_HERMES_HOME` and `AIGW_ACCEPTANCE_CLIENT_PATH`. Windows also
  exports `CLAUDE_CODE_GIT_BASH_PATH`.
- All supply state lives in one job-owned directory:
  - homes, caches, temporary files and the Hermes launcher are inside it;
  - a failed or interrupted supply removes it;
  - retirement removes only that recorded directory and proves it absent;
  - on Linux and macOS, retirement keeps a directory that a live process still
    uses and fails the job.
- The GitHub manual input `windows_clients` becomes `native_clients`. Every
  native job declares the same supply and retirement steps and the same client
  tool closure.
- One pin record in the pipeline model selects the client releases and the
  GitLab Windows supplier name. Both Forge projections are regenerated from it.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: real-client acceptance supplies the official
  clients that each native host's journey declares, from one pin record, in
  job-owned state that every outcome retires.

## Impact

`tools/ci` client supply and retirement, the CUE pipeline model, both generated
Forge projections, their projection tests and `CONTRIBUTING.md` change. Manual
GitHub dispatches that used `windows_clients` must use `native_clients`.
Windows supply keeps its packages, isolation and environment. GitLab keeps its
Windows public input; real-client supply in GitLab's Linux and macOS jobs stays
out of scope. No product behavior, configuration, credential or release
artifact changes.
