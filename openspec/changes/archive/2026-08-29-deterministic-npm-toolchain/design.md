## Context

See [proposal.md](proposal.md). mise locks the selected npm package version but
does not carry that package's complete npm dependency graph. npm already
provides the standard portable lock and clean-install mechanism for this
closure.

## Goals / Non-Goals

**Goals:**

- One dependency authority for each ecosystem.
- Identical npm tool bytes on clean local, GitHub, and GitLab executions.
- One CUE source for both Forge projections.

**Non-Goals:**

- A new package manager, installer, wrapper, cache, or trust exception.
- Changes to AIGW runtime dependencies or client behavior.

## Decisions

### Use npm's native lock authority

`package.json` declares the three npm repository tools and
`package-lock.json` owns their complete closure. `npm ci --ignore-scripts`
materializes exactly that graph. This replaces, rather than supplements, the
three mise npm declarations. pnpm, Pixi, and a custom downloader add another
tool or implementation without improving this small Node-only closure.
The repository `.npmrc` selects the canonical npm registry so user-level npm
configuration cannot rewrite lockfile origins or signature-key discovery.

### Keep execution topology in CUE

The source jobs install the Node runtime and binary tools through mise, then
materialize npm tools before invoking the existing source gate. GitHub and
GitLab remain projections of `.config/ci/pipeline.cue`; neither gains an
independent workflow implementation.

### Expose local bootstrap instead of reinstalling on every gate

Contributors run the same `npm ci --ignore-scripts` bootstrap after
`mise install --locked`. The source gate consumes `node_modules/.bin` through a
portable process environment and does not repeatedly delete and rebuild the
closure during every focused check.

## Risks / Trade-offs

- **Risk:** the local npm closure is absent or stale. **Mitigation:** contributor
  setup is explicit and CI always performs a clean locked installation.
- **Risk:** a host-level registry override changes package or key discovery.
  **Mitigation:** the repository owns the canonical registry selection and the
  lock retains package integrity for every installed archive.
- **Risk:** dependency authority is described inconsistently. **Mitigation:**
  update the authority map and release contract in the same atomic change.

## Migration Plan

Remove the mise npm declarations, generate the npm lockfile, update the CUE
model and projections, and verify from an isolated empty npm cache. Rollback is
the atomic commit revert; no product data migration is involved.
