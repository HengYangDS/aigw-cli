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
  pinned 2026.11.7. The refresh of 2026-11-14 held the same release and set its review for 2026-11-17; this
  refresh is that review, and no window was held in between. The CUE authority derives one Mise release from the Linux image's tag and pins the digests
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
  `runner-mise-window` will move them together with its window; it is not open yet. Owner: the maintainer of
  this repository's CI guests. Date: 2026-11-20, the day on which the window for the rollback copies is due.
  The Mise action of the hosted runners is a separate pin and advances here.

### The YAML library's new import path gets a Change of its own

- **Rests on:** the calculation of 2026-11-16, which reports `go.yaml.in/yaml/v4` v4.0.0 for
  `go.yaml.in/yaml/v3` v3.0.6. In Go a major version is a module of its own: v3.0.6 is the newest release of
  the module that this repository pins, and v4 is another module, which 34 Go files at this lane's head
  would import in its place, the writer of a client's configuration among them. The refresh of 2026-11-14
  held it and set a review for 2026-11-17; this refresh is that review. A reading of v4.0.0's source on
  2026-11-16 found that its encoder folds a line past column 80 where v3 folds none, and that its node type
  gains a field that would enter a state file which released programs read with unknown fields refused.
  No test of this repository compares a written file byte for byte.
- **Considered:** migrate in this refresh; migrate in a Change of its own after this one.
- **Chosen because:** the supply rule asks for the newest release of each pinned input, and v3.0.6 is that.
  A move to another module that changes the bytes of a user's file is a change of the product: it needs
  tests of the written bytes first, which do not exist, and it should not decide when the fixes that already
  wait on `dev` reach users.
- **Applies to:** `go.yaml.in/yaml` only. The Change `yaml-v4-import-path` will carry the migration; it is
  not open yet. Owner: this repository's maintainer. It opens when this Change has landed. Date: 2026-11-18.

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
