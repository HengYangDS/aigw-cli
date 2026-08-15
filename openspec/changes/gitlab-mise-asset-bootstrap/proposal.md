## Why

GitLab Linux still fails after the initial HTTP/1.1 retry fix because the
downloaded upstream installer performs a second uncontrolled curl request.
The repository must own the entire bootstrap transport and verify the exact
locked mise asset before execution.

## What Changes

- Derive the exact mise release and Linux architecture from repository and
  runner facts.
- Download the official release asset and checksum manifest directly with
  bounded HTTP/1.1 requests.
- Verify the selected asset checksum before extracting and installing it.
- Keep `.config/ci/pipeline.cue` as the only command authority and regenerate
  the GitLab projection.

## Impact

GitLab Linux bootstrap no longer delegates an unbounded nested download to an
installer script, while the tool version remains locked by `mise.toml` and no
parallel CI script is introduced.
