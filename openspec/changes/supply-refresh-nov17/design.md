## Context

See [proposal.md](proposal.md). The sweep read every input class at `9a3cf734` on 2026-11-16: Renovate's
candidate calculation under this repository's policy reported five updates and none held back by the release
age; the Mise tools, the native clients (Codex 0.169.0, Claude Code 2.1.325) and Hermes v0.23.1 are at their
newest stable release.

## Goals / Non-Goals

**Goals:**

- Every candidate is taken, or held with its owner, its next action and a date.
- The unreleased changes of `dev` reach users as 0.8.1.

**Non-Goals:**

- A window on a CI guest.
- The migration to a new import path of the YAML library.

## Decisions

### Mise 2026.11.8 waits for the runners' window

- **Rests on:** the calculation of 2026-11-16, which reports `docker.io/jdxcode/mise` 2026.11.8 against the
  pinned 2026.11.7. The CUE authority derives one Mise release from the Linux image's tag and pins the digests
  of the macOS and Windows executables that the GitLab runners own; each macOS and Windows job verifies its
  executable against them before it runs. The executables of 2026.11.7 were placed on both guests on
  2026-11-13, and the Windows replacement needed the guest's network and two paused runners. The scenario "A
  refresh has no window for a runner-owned tool" states what a refresh does without a window.
- **Considered:** hold a window during this refresh; advance the Linux image alone; keep the release.
- **Chosen because:** the image cannot move alone, since the three platforms verify one release. A window
  on the shared Windows guest is already owed for the rollback copies of 2026-11-13, and one window can serve
  both; a second one a few days earlier costs the other projects' runners an interruption for one patch
  release.
- **Applies to:** the Mise release of the Linux image and of the two runner-owned executables. The Change
  `runner-mise-window` moves them together with its window: owner, the maintainer of this repository's CI
  guests; date, 2026-11-20. The Mise action of the hosted runners is a separate pin and advances here.

### The YAML library's new import path gets a Change of its own

- **Rests on:** the calculation of 2026-11-16, which reports `go.yaml.in/yaml/v4` v4.0.0 for
  `go.yaml.in/yaml/v3` v3.0.6. The module path changes, and 33 Go files of this repository import the
  library, among them the readers and writers of client configuration files.
- **Considered:** migrate in this refresh; migrate in a Change of its own after this one.
- **Chosen because:** a major release of the library that reads and writes users' client files needs its
  release notes read against every use, and its output compared byte for byte, before it ships. That review
  should not decide when the fixes that already wait on `dev` reach users.
- **Applies to:** `go.yaml.in/yaml` only. The Change `yaml-v4-import-path` carries the migration: owner, this
  repository's maintainer; it opens when this Change has landed; date, 2026-11-18.

### 0.8.1 is cut with this refresh

- **Rests on:** the changelog's unreleased section at `9a3cf734`: each command asks the credential store
  about a credential once, and `aigw use` says that an entered Token was not kept when its selection fails,
  a sentence that 0.7.0 to 0.8.0 lack while the Token itself is removed correctly. A refresh was due on
  2026-11-17 in any case.
- **Considered:** a release of its own on 2026-11-16; a release with the refresh.
- **Chosen because:** nothing is lost or exposed without the sentence, and a second release route a day
  earlier would hold the workstation's one verification queue for several hours.
- **Applies to:** the version and the changelog heading of this lane. The publication of the release is not
  a task of this Change.

## Risks / Trade-offs

- [A held candidate is forgotten] → Each hold names its Change, its owner and its date above, and the next
  refresh's calculation reports both candidates again.

## Migration Plan

None. No pin that a user's installation reads changes its meaning.
