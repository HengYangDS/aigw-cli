## Context

The first forward fix controlled only the request that downloaded
`install.sh`. Hosted evidence showed that the script's own release-asset curl
could still negotiate HTTP/2 and fail with `PROTOCOL_ERROR`. Environment
variables intended for mise do not control that installer-internal curl.

## Decision

- Read the exact mise version from the existing `mise.toml` authority.
- Map the native Linux machine architecture to the official asset name.
- Fetch the exact archive and `SHASUMS256.txt` with bounded HTTP/1.1 requests;
  archive retries resume the verified target file instead of discarding bytes
  already received on the slow GitLab runner path.
- Select exactly one checksum entry, normalize its relative filename, verify it
  with `sha256sum`, then extract and install the verified executable.
- Keep the command inline in the CUE model because extracting a standalone
  shell program would create a second CI command surface for one bounded step.

## Rejected alternatives

- Re-running `install.sh`: preserves the uncontrolled nested request.
- Calling the GitHub Releases API: adds anonymous API quota and Forge coupling.
- Duplicating the bootstrap in a tracked shell script: adds a parallel owner
  without reducing product or maintenance complexity.
