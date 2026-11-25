## Context

The proposal gives the gap. Three resolvers can each hold a release age: Mise (`[settings] minimum_release_age`), Renovate (the policy's root `minimumReleaseAge`) and npm (`min-release-age`). Only Renovate's is written, at 0 hours (`.config/checks/renovate/renovate.json5`, read 2026-11-25). Mise reads `minimum_release_age` today from the host's user configuration (24h on this host, read with `mise settings ls`), which declares nothing for the repository.

## Constraints

Every option was checked against these before the decision.

1. Every repository that ETHOS governs holds a new release for 24 hours, writes that age itself and lets a security fix skip it (owner, 2026-11-19, relayed).
2. The age is one value, in whole hours, edited in one place; tests read where and in which unit it is held, never how long it is (owner, 2026-11-13, relayed; accepted `supply-chain`, scenario `The minimum release age changes`).
3. The check joins the quality graph, so the full local proof runs it with no remote and both Forges run it from the same graph (`AGENTS.md`).
4. One owner per fact: the age is authored once and every other setting is derived from it.
5. No new Go dependency without necessity: `go.mod` has no JSON5 reader (read 2026-11-25).

## Goals / Non-Goals

**Goals:** one authored release age, in whole hours; Renovate's and npm's settings hold it; a later change of the age is one edited line and one command; the local proof and both Forges check it once.

**Non-Goals:** a release-age reader of ETHOS; any dependency's version; the proxy repository; the value of the age, which the owner set at 24 hours.

## Decisions

### The declaration is Mise's own setting

- **Rests on:** measured on 2026-11-25 in a disposable folder with Mise 2026.11.13: a project line that sets `minimum_release_age` to 48h made `mise settings get minimum_release_age` print 48h while the host's user configuration holds 24h, so a repository line wins over the host's. Relayed, not measured here (ETHOS session 97f828, Mise 2026.11.9, 2026-11-18): the setting does not filter an exact version or one in `mise.lock`. Every tool of `.config/mise/config.toml` is declared at an exact version and locked (read 2026-11-25), so the line enters without moving a lock.
- **Considered:** Renovate's root value as the declaration, as the accepted specification says today (nothing but Renovate reads it); a new file that holds the hours; a Mise `[vars]` entry.
- **Chosen because:** Mise's setting is itself read by a resolver, so the declaration is never a number that nothing enforces, and the proxy repository has held this form since its commit `4e49af33` (2026-11-21).
- **Applies to:** `.config/mise/config.toml`.

### Renovate's and npm's lines are filled, not compared

- **Rests on:** npm 12.2.3 defines `min-release-age` as null or Number, counted in days (its definitions and documentation, read 2026-11-25). `npm config get` prints 1, 0 and 1.5 unchanged, so npm checks no unit, and with the value 1 it writes nothing to stderr (measured 2026-11-25, disposable folder). The conversion from hours to days is therefore this repository's rule.
- **Considered:** a test that compares authored numbers (a second number to keep equal); Mise `[env]` entries, which reach npm only under Mise and leave Renovate out; filling by hand.
- **Chosen because:** one function gives each setting's line for a number of hours, and the fix and the check both use it, so the check cannot drift from the fill. Renovate takes `<hours> hours`; npm takes the days that cover the hours, rounded up, which never takes a release that the declared hours hold; an age of zero leaves npm's line out.
- **Applies to:** `tools/ci` (the `release-age` fill and the `check-release-age` check), the `fix` task of `.config/mise/config.toml`.

### Renovate's JSON5 is read by the parser the tests already use

- **Rests on:** `go.mod` has no JSON5 reader; `tools/ci/dependencies_test.go` reads the policy with Prettier's JSON5 parser through Node, and `tools/ci/format.mjs` and `tools/ci/markdown/lint.mjs` are Node helpers that Go starts (read 2026-11-25). The policy's keys are unquoted; its root age stands on one line at two spaces (`renovate.json5:33`) and the security-fix rule's own age is nested deeper (`:42`).
- **Considered:** a Go JSON5 dependency (new supply for one value); a regular expression alone, which cannot show that Renovate reads the line as the root value; an own parser.
- **Chosen:** Go replaces the one root line; a Node reader, `tools/ci/release-age.mjs`, takes the declared hours as its argument and the filled text on stdin, reads the root age, the security-fix rule's age, the strict internal-checks filter, the behaviour for a release without a publish time and every other age or `extends` entry as the JSON5 parser does, and prints each difference from the declaration on stderr; a key declared twice is one of them. The fill reads every file, fills it in memory, has the reader read the result and opens each file that differs for writing before the first write, so a refusal or an unwritable file writes nothing; a failure inside the writes themselves is not rolled back.
- **Applies to:** `tools/ci/release-age.mjs`, `tools/ci`.

### A file that carries another release rule is refused

- **Rests on:** in npm, `before` wins over `min-release-age` within one source and `min-release-age-exclude` exempts packages from it (documentation, read 2026-11-25); the accepted scenario already fails the checks while another rule declares an age of its own.
- **Chosen:** the check names an npm line with `before`, a second `min-release-age` or any `min-release-age-exclude`, and a Renovate age other than the root's and the security-fix rule's.
- **Applies to:** `tools/ci`.

### The check is a gate of the quality graph for two carrier classes

- **Rests on:** `validateQualityGraph` needs each gate to have a carrier consumer and each class's gates to cover its concerns, and it accepts further gates in a class (read 2026-11-25, `tools/ci/quality.go`). Renovate's policy belongs to `dependency-policy`; the Mise and npm configuration belong to `toolchain`.
- **Chosen:** gate `release-age`, listed by both classes, so the local proof and both Forges run it once.
- **Applies to:** `tools/ci/quality.go` and the tests that list the quality commands.

## Risks / Trade-offs

- npm knows no advisory: under the age, `npm audit fix` keeps a vulnerable version, warns and exits non-zero when the fix is younger (documentation, read 2026-11-25). A security fix younger than the age is taken in its own change, which says so; Renovate's security rule stays at 0 hours.
- `npm ci --ignore-scripts` under the filled line ran once on 2026-11-25 at 09:50 in an owned scratch folder holding the lane's `package.json` and `package-lock.json`, with an empty cache. `npm config get min-release-age` read 1 through the lane's Mise environment; the install added 116 packages, found 0 vulnerabilities and exited 0. A passing `npm ci` says nothing about the lock's age (ETHOS session 97f828, 2026-11-18, relayed), so this shows only that the line does not break the install.
- Above zero, Renovate adds a buffer of 30 minutes (the proxy's governance document, read 2026-11-25); the accepted scenario `A registry publishes no release time` covers a registry without release times.

## Open Questions

- npm's value is one day, because the owner's rule names every source. Keeping npm longer would end the one-value requirement and is the owner's decision.
- If ETHOS's base later fills these lines from Mise's setting, AIGW adopts that and keeps no second copy.
