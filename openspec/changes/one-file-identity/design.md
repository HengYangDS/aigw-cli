## Context

`client-chosen-model` landed in `dev` at `06edce9a4` on 2026-11-23. Its twelfth review (2026-11-22, brief
`build/runtime/work/review-12-chosen/brief.md` in the canonical checkout, probes `zz12_review_*_test.go` beside it)
named two facts that each have two sources. This Change removes the first: "which of a client's files does a
transition drop". The review's record is `build/evidence/selection-states-20261121/README.md`, section "Version twelve
and the twelfth review".

## Goals / Non-Goals

**Goals:** one owner of the judgement "do these two configured paths name one file"; every decision about kept and
dropped client files made by it; the review's findings 1 and 4 failing first and passing after.

**Non-Goals:** the second root cause (whether a Token is usable when its store fails), which is its own Change;
changing what AIGW writes into a file; Claude Desktop, whose target is a library folder from which its files are
derived, so that two spellings of one folder can derive different files, and which `client-chosen-model` does not
cover; Hermes's write replacing a symbolic link to its file with a regular file, a separate defect found on
2026-11-23 and filed apart.

## Decisions

### The judgement lives in `internal/platform`

- **Rests on:** `aigw client edit` (`internal/cli/client`), synchronization (`internal/app/synchronization`) and both
  adapters already import `internal/platform`, and the judgement needs the file system, which `internal/domain` may
  not use (the architecture gate).
- **Considered:** an exported function in `internal/adapter`; a method on `domain.ClientBinding` that takes a stat
  function.
- **Chosen because:** one package that every caller already depends on, with no new dependency direction; the
  adapters keep their own reasons for resolving paths (Codex's canonical path, its state file) and ask only this
  question of it.
- **Applies to:** `platform.SameFile` (Codex's `sameTargetFile` and its `statTarget` hook, moved),
  `platform.CanonicalPath` (Codex's `canonicalCodexTargetPath`, moved) and `platform.DroppedFiles`, which replaces
  the four string comparisons. The implementer found a third judgement of the same fact in discovery
  (`sameSurfacePath` in `internal/adapter/discovery/surfaces.go`, which compared a missing file's absolute path) and
  it now asks `platform.SameFile` too; without it, a default home named through a linked folder was not found with
  the Token. As a consequence a hard link or a name that differs only in case on a volume that ignores case now
  names the same surface, as the rule says.

### A path that does not exist compares by its resolved path

- **Rests on:** a target that a command creates has no identity before it exists; Codex already resolves a target's
  path by resolving the symbolic links of the folders that exist and keeping the names of the parts that do not
  (`canonicalCodexTargetPath`).
- **Considered:** comparing the cleaned spelling, Codex's rule for `sameTargetFile` until now. The implementer found
  on 2026-11-23 that this leaves two judgements for a missing file named through a linked folder: Codex's union
  compares resolved paths and the dropped-file judgement would compare spellings.
- **Chosen because:** one canonical form, moved from Codex into `internal/platform`, serves both, so a missing file
  is judged alike everywhere.

### Hermes writes through a link to its file

- **Rests on:** the owner's word of 2026-11-23 17:01 ("Do this task here") on the defect that Hermes's atomic write
  (a rename over the configured path) replaces a symbolic link to its file with a regular file and leaves the link's
  target unchanged; Codex resolves its target before it writes and does not have it.
- **Chosen because:** with the one canonical form in `internal/platform`, Hermes writes the file that the link
  names, so the configured path and the written file stay one file, which this Change's requirement asks for.

### A missing file compares its existing folder by identity

- **Rests on:** the fresh review of `84f6fa5eb`: on a volume that ignores case, a missing file in a folder named in
  another case was two files by its canonical spelling, while the same folder is one by the operating system; the
  refusal's reason then followed the Token.
- **Chosen because:** comparing the deepest existing folder by the operating system's identity and the names below
  it by spelling makes links and case one judgement for a missing file too.

## Risks / Trade-offs

- **A target named by a hard link** is one file by the judgement, but AIGW's record of a file is kept by its name, so
  such a target finds no record and is refused, with the Token and without it. Hard-linked configuration files are
  rare; this Change does not move records to file identity.
- **`..` after a symbolic link** is cleaned by its spelling before any judgement (`filepath.Abs`), as Go and AIGW
  did before this Change: `link/../codex/config.toml` names `codex/config.toml` beside the link, while the operating
  system opens the file beside the link's target. Every part of AIGW sees the same cleaned path, so the outcome does
  not depend on the Token; a configured path that relies on `..` through a link is written where its spelling says.
- **A stat per comparison:** client target lists hold one to a few files; the cost is negligible.
- **One error message changes:** a loop of symbolic links in a target now reads `resolve symbolic links: …`, not
  `resolve Codex target symlinks: …`; the path that fails is still named first.
- **A file replaced between two stats** (another program renames over it) is compared by the identity it has when
  read; each command reads its targets once, as today.
