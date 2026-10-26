## Why

The second run of the 0.4.17 proposal
([HengYangDS/aigw-cli#206](https://github.com/HengYangDS/aigw-cli/pull/206))
ran Mise 2026.10.19 on every Forge job and passed Windows. Native Linux and
macOS still failed `TestGitLabUnixMiseMirrorFailureStaysOnSelectedPeer`. On
hosted runners Mise colors its diagnostics, and the test matched the literal
text `mise WARN` in lines whose words were separated by escape sequences. The
expected warning was printed but not counted. Local runs print no color, so
they passed.

## What Changes

- The mirror failure test judges Mise's diagnostic text after removing terminal
  escape sequences, as the native diagnostic classifier already does.
- A regression test feeds the colored output that the hosted runner recorded,
  and requires exactly the one expected warning.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: assertions about native tool diagnostics judge their text,
  independent of terminal presentation.

## Impact

Only `tools/ci/projection/mirror_failure_test.go` changes. No product,
pipeline or dependency behavior changes.
