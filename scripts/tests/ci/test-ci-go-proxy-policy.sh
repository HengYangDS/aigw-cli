#!/bin/sh
# Verify the GitLab module-source fallback semantics without contacting a proxy.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)

expected='${AIGW_GOPROXY:-https://goproxy.cn|https://proxy.golang.org|direct}'
obsolete='${AIGW_GOPROXY:-https://goproxy.cn,direct}'
grep -Fq "$expected" "$root/.gitlab-ci.yml" || {
  echo "GitLab GOPROXY default must use a pipe-separated resilient fallback chain" >&2
  exit 1
}
if grep -Fq "$obsolete" "$root/.gitlab-ci.yml"; then
  echo "GitLab GOPROXY must not leave a comma-separated timeout-terminal fallback" >&2
  exit 1
fi
echo "GitLab Go proxy fallback policy: OK"
