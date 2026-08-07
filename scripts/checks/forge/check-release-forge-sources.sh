#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example \
  AIGW_GITLAB_RELEASE_REPOSITORY=group/aigw-cli \
  AIGW_GITHUB_RELEASE_ORIGIN=https://github.example \
  AIGW_GITHUB_RELEASE_REPOSITORY=organization/aigw-cli \
  go run "$root/tools/releasekit" validate-release-sources

echo "release forge-source contract: OK"
