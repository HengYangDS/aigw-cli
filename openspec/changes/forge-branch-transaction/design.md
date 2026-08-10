## Context

AIGW already projects GitLab history into the GitHub identity domain, but that
operation is remote-facing and intentionally does not advance local `main`.
Using raw Git or overloading Forge projection would create a second, implicit
release protocol.

## Decision

`go run ./tools/forge promote-release` is the only repository-owned local
release promotion. It requires the exact observed `main` and `dev` commits,
proves that `main` is an ancestor of accepted `dev`, and updates `main` with one
compare-and-swap. Equal refs are an idempotent success. Dirty, stale, missing,
or diverged state fails before mutation.

Remote publication remains separate:

| Transition | Owner | Effect |
| --- | --- | --- |
| `candidate/dev` to `dev` | ETHOS | accepted integration |
| `dev` to local `main` | `tools/forge promote-release` | exact local release CAS |
| local canonical to GitLab | Git push/release pipeline | GitLab publication |
| local canonical to GitHub | `tools/forge project` | signed identity projection |

## Verification

1. Observe a focused test fail before production implementation exists.
2. Prove success, stale-main, stale-dev, divergence, dirty-state, and idempotent
   behavior.
3. Run the complete native source, architecture, repository, and coverage gates.
4. Execute exact-HEAD ETHOS proof, archive, land, and close out before release.
