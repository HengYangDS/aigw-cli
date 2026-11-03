## Why

The GitLab Windows ARM64 host keeps one runner-owned Mise in
`C:\Program Files\mise\bin`, and four projects run it. AIGW pins that
executable's digests in its CUE authority, while other projects use it
unpinned. An AIGW Mise refresh therefore replaces a binary that other projects
run, and their refreshes can break AIGW's digest check. On 2026-11-02 the user
chose one Mise directory per project, with AIGW first.

## What Changes

- AIGW's GitLab Windows jobs run the Mise executable and shim from
  `C:\Program Files\mise-aigw-cli\bin`, named once in the CUE authority; the
  digest and version checks stay as they are.
- The 2026-11-03 Windows maintenance window installed that directory with the
  pinned Mise 2026.10.25 executable and shim, both digests verified, and left
  the shared directory unchanged.
- The projection test and the retained-step record of the Windows Mise
  preparation follow.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: on a Windows host that several projects share, AIGW's
  runner-owned Mise lives in a directory that only AIGW's declaration updates.

## Impact

The CUE authority, the GitLab projection, its projection test and the
retained-step record change. GitHub's jobs and the released programs do not, so
no release follows. The other projects keep the shared directory; moving them
is their owners' decision. Out of scope: the total concurrency cap per VM.
