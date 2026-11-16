## Context

See [proposal.md](proposal.md). Release construction runs GoReleaser's snapshot build in the checkout
(`tools/release/construction`). It already gives GoReleaser Git configuration through the process
environment when a checkout has several peers and no `origin`.

## Goals / Non-Goals

**Goals:**

- The build is complete in a checkout without a Git remote, as the scenario "No Forge is configured"
  requires, and its artifacts name the commit that they were built from.

**Non-Goals:**

- A change to which native diagnostics prevent qualification.
- A change to the build of a checkout that has a remote.

## Decisions

### Where no Forge is configured, the checkout itself answers for the remote

- **Rests on:** a measurement of 2026-11-16 in a clone of `9a3cf734` with its remote removed (GoReleaser
  2.20.2, Git 2.56.1, Git's trace to a file). GoReleaser asks Git one question about the remote,
  `git ls-remote --get-url`. Without a remote Git fails, GoReleaser writes "ignoring errors because this is a
  snapshot … couldn't get remote URL", the common runner counts that line as a native diagnostic, and
  GoReleaser's metadata names the commit `none`. With the URL `.` in `remote.origin.url`, given through the
  process environment, the line is gone, the metadata names the real commit, and the clone's Git
  configuration is unchanged. An absolute path of the checkout behaves the same.
- **Considered:** refuse a checkout without a remote and name the fix; hold that one warning harmless; give
  GoReleaser the product's public repository as the URL; give it the checkout itself.
- **Chosen because:** a refusal contradicts the accepted scenario, which requires a complete build with zero
  remote peers. A warning held harmless would be a second authority on diagnostics, which "Warnings are owned
  failures" rules out, and the placeholder commit would stay. The public repository's URL names a Forge where
  none is configured. The checkout is the local product-object authority that the requirement names; `.` is
  that checkout from the directory in which GoReleaser runs, the same on every platform and without a path
  of the host.
- **Applies to:** a checkout with no Git remote, on a branch or detached. A checkout with a remote gets what
  it got before: nothing where Git already answers, and the selected peer where several peers and no `origin`
  exist. Construction writes no Git configuration.

## Risks / Trade-offs

- [GoReleaser reads the URL for more than its metadata in a later release] → The build test runs the real
  GoReleaser with this repository's configuration in a checkout without a remote on each platform, so a new
  diagnostic fails it.

## Migration Plan

None.
