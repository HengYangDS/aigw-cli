## Context

The review ran in a fresh context over `b1a05033..5b6143f9`. Each finding was checked against the sources before
this Change: all eight hold. Two statements in archived documents are wrong and stay as history; this design
corrects them.

## Goals / Non-Goals

**Goals:**

- Close each finding at its owner, with a test where a test could have caught it.
- Correct the two wrong statements in the living specifications.

**Non-Goals:**

- Changing a product deadline. The five-second, one-second and 1.5-second native bounds keep their review of
  2026-11-20.
- Rewriting archived Changes.

## Decisions

### The release tool's Keychain tests share the hang bound's value

`tools/release` gets its own named 30-second bound. It cannot import the native package's test constant, and one
line per package is cheaper than an exported test helper. `secret-storage` already states the rule for every test
that drives `/usr/bin/security`.

### An unclassified family of a curated vendor fails

The line test names each family that starts with `claude-`, `gpt-` or `gemini-` by its vendor and product-line
word. A family of those vendors that yields no line, such as `gpt-7` or `claude-5-6-opus`, fails instead of being
skipped, and a suffix after the line word, such as `gpt-6.2-astra-1120`, still counts for that line. The vendor
word counts wherever it stands and however it is capitalized. A model of a curated family carries that family's
id, with at most a channel suffix, so a successor cannot be filed under the family it replaces.

### The preflight owns the word `mise`

A runner account whose login profile activates Mise defines a shell function `mise` that calls the shared
executable by absolute path. Putting a directory first on `PATH` does not change a function. A read of the macOS
host on 2026-11-13 found exactly that in the profile of AIGW's review runner account, and not in the protected
one. So since the preflight landed, review jobs verified AIGW's Mise and installed with it, but ran
`mise exec` through the shared Mise, which held the same release. The preflight therefore removes any such
function, requires `command -v mise` to name the verified file, and prints the verified release.

The account's profile line is host configuration and leaves in the next host window; the preflight does not
depend on it. The same activation also wraps `cd` and defines a command-not-found handler, both bound to the shared
Mise. The preflight leaves those: no macOS job changes directory in its script or relies on a missing command, and
they go with the profile line.

A function that the step cannot remove, such as a read-only one, still owns the word, so the step stops there. An
executable that does not start now says so; under `set -e` it had ended the step without a message.

A test on macOS runs the generated step against a stand-in executable: a matching file passes and leaves `mise`
bound to it even when a function was defined; a function that cannot be removed, a missing file, another digest,
another version and an executable that does not run each stop with their own message. The earlier test compared
text only. The text test also refuses a top-level `before_script` and a macOS job that extends a template, either
of which could run a command before the preflight.

### The lane's own review

A second fresh-context review, of this lane, found that two of its new tests could not fail for what they claimed.
No executed case reached the stop after `command -v`, so removing its `exit 1` passed every test. And the line test
still skipped a curated vendor's family when the vendor word was spelled or placed differently, such as
`GPT-6.2-astra` or `openai-gpt-6.2-astra`, and never read the models filed under a curated family. Both are closed
here, each with a mutation that fails; the review's findings on the release age are closed in `release-age-floor`.

### Hermes release tags have a major below 1000

The publisher also carries calendar tags such as `v2026.9.24`. The rule's expression accepted them, and only
Renovate's default limit on major jumps dropped them. The expression now takes at most three digits for the major.

### Corrections of archived statements

- `keychain-fixture-bounds` said that the child test process keeps the product's five-second deadline. It does
  not: the child calls the native functions in-process. A real Keychain operation that takes 5 to 30 seconds
  passes those tests, while the product stops its worker at five seconds. A prompt that waits for a person still
  fails.
- `mise-2026-11-7` said that another owner updates the shared macOS Mise and that six projects share the host.
  Runner jobs of eight projects share it, and the CI infrastructure owner updates that Mise; no single project's
  declaration governs it.

## Risks / Trade-offs

- The executed preflight test needs `shasum` and a POSIX shell, so it runs on macOS only, where the step runs.
