## Why

Since 2026-11-19 every repository that ETHOS governs holds a new release for 24 hours and writes that age itself; a security fix skips it (owner, relayed on 2026-11-19). AIGW writes only Renovate's root `minimumReleaseAge`, at 0 hours. Mise's configuration has no `minimum_release_age` and the npm configuration has no `min-release-age`, so a lock refresh by hand, a tool that Mise resolves and an npm resolution follow no age.

The accepted requirement `Latest stable repository-owned supply chain` already asks for one minimum release age, in hours, for every source, and its scenario `The minimum release age changes` says that every source follows one value. The repository meets that for Renovate alone. The proxy repository has held the full form since its commit `4e49af33` (2026-11-21): one declaration in Mise, the other resolvers' settings filled from it.

## What Changes

- The repository declares its release age once, in whole hours, as Mise's own `minimum_release_age` in `.config/mise/config.toml`, at 24 hours.
- A check of the quality graph, written in Go beside the other `tools/ci` checks, reads that declaration and requires Renovate's root `minimumReleaseAge` and npm's `min-release-age` to hold it. npm counts whole days, so it holds the days that cover the declared hours, and an age of zero leaves its line out. The check refuses a file that carries another release rule of its own. `mise run fix` writes those two lines from the declaration.
- Renovate's security-fix rule stays at 0 hours.
- The accepted requirement and its scenario say where the one value is written and that the other settings are filled from it.

No behavior of the product changes. Not breaking.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `supply-chain`: the one release age is Mise's declaration, and Renovate's and npm's settings hold it.

## Impact

`.config/mise/config.toml`, `.config/npm/npmrc`, `.config/checks/renovate/renovate.json5`, `tools/ci` (the check, the quality graph, the `fix` task and their tests), `docs/governance/dependency-maintenance.md` (the dependency maintenance section moves there from `docs/governance/change-and-release-policy.md`, which stands at the 500-line ceiling) with the links to it in `docs/governance/README.md` and `CONTRIBUTING.md`, and the accepted `supply-chain` specification. The design records what Mise and npm do under the age, measured in a scratch copy before the two lines are written.

Reuse: the proxy's form is taken over, its Python module is not, because `tools/ci` is Go and Renovate's policy here is JSON5 with unquoted keys.

Out of scope: ETHOS's own release-age reader (if its base later fills these lines, AIGW adopts that and keeps no second copy); any dependency's version; the proxy repository; the value of the age, which the owner set at 24 hours.
