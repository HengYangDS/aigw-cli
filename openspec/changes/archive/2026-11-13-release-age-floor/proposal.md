## Why

AIGW's dependency policy declared release ages in three places: three days at its root, which governed npm, and two
rules that set the age to zero for Go modules, Mise tools, Actions and images. Nothing stated the rule as one value,
so changing it meant editing several rules and a specification that named each age.

On 2026-11-13 the owner decided the rule for every governed repository: the release age is one configurable value
per repository, in hours, easy to change, and it is zero. Earlier that evening this Change had held every source for
one day; that step is superseded here and stays in the lane's history.

The morning's refresh had also adopted markdownlint 0.42.1, which requires the fixed re2js itself, and left the
override in place that `supply-chain` says must leave with it.

## What Changes

- Declare the release age once, at the root of the dependency policy, in hours. It is `0 hours`: a stable release
  may be taken at publication. No package rule declares an age of its own.
- Keep the rule that a security fix skips the age, whatever the age is.
- Let lock maintenance run at any time instead of Renovate's weekly default, so transitive releases follow the same
  age.
- Test where the age is declared and in which unit, not how long it is, so that raising it is a one-line change.
- State the declared age in the `supply-chain` requirement, and what happens above zero when a registry publishes
  no release time.
- Remove the re2js override that markdownlint 0.42.1 made unnecessary.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: the dependency policy declares one release age in hours for every source, and the repository's
  checks fail while another rule declares its own.

## Impact

The dependency policy, its test, the `supply-chain` specification, the dependency section of the change and release
policy, and the npm manifest. No product code changes. npm releases, which waited three days, may now be proposed at
publication; each still needs its own admission, checks and proof.
