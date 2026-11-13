## Why

An independent review of the five Changes of 2026-11-13 found one medium and seven low defects in work that had
passed every gate. The medium one: the Keychain fixture bound was raised in `internal/secret/native` only, while
`tools/release` keeps two five-second bounds around the same `/usr/bin/security` calls, so the failure of job
69467 can still occur there. The others are a restated requirement that the CHANGELOG contradicts, a team-file
test that skips what it cannot classify, a stale sentence in the rollout guide, a macOS Mise preflight that a shell
function bypasses in review jobs and whose test reads only text, and a Hermes tag rule that depends on a Renovate
default.

## What Changes

- Bound the release tool's own Keychain fixture calls by 30 seconds, like the native package's tests.
- Make the team-file line test fail for a Claude, GPT or Gemini family that names no known line.
- Correct the rollout guide's sentence on which model generations appear.
- Make the macOS Mise preflight bind the word `mise` to the verified executable and print what it verified, and
  test it by running it.
- Read only Hermes release tags with a major below 1000, so calendar tags are not candidates.
- Restate two scenarios that were wrong: which pre-release candidates keep a CHANGELOG section, and who updates
  the shared macOS Mise.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `release-distribution`: a candidate keeps its own CHANGELOG section only under its published tag.
- `supply-chain`: on a shared macOS runner host the word `mise` runs the verified executable for the whole job.

## Impact

Test files, the CI pipeline model with its generated GitLab projection and retained-step record, the dependency
policy, one guide and two specifications. No product code changes. The macOS GitLab jobs print one more line.
