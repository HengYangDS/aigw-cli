#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
checker="$root/scripts/checks/forge/check-release-tag-signature.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-tag-provider.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=core.hooksPath
export GIT_CONFIG_VALUE_0=/dev/null

git init -q "$tmp/repository"
git -C "$tmp/repository" config user.name 'AIGW tag provider fixture'
git -C "$tmp/repository" config user.email 'fixture@example.invalid'
printf 'fixture\n' > "$tmp/repository/fixture"
git -C "$tmp/repository" add fixture
git -C "$tmp/repository" commit -qm fixture
git -C "$tmp/repository" tag -a v0.0.0-unsigned -m unsigned
git -C "$tmp/repository" update-ref \
  refs/tags/github/v0.0.0-unsigned refs/tags/v0.0.0-unsigned
printf 'fixture trust input\n' > "$tmp/allowed-signers"

if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed-signers" \
  sh "$checker" "$tmp/repository" github/v0.0.0-unsigned gitlab \
    >"$tmp/wrong-provider.out" 2>&1; then
  echo 'qualified GitHub tag was accepted under the GitLab provider' >&2
  exit 1
fi
grep -F 'qualified GitHub tag requires github provider' \
  "$tmp/wrong-provider.out" >/dev/null

if AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed-signers" \
  sh "$checker" "$tmp/repository" github/v0.0.0-unsigned github \
    >"$tmp/github.out" 2>&1; then
  echo 'unsigned qualified GitHub tag was accepted' >&2
  exit 1
fi
grep -F 'release tag is not SSH signed' "$tmp/github.out" >/dev/null

if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed-signers" \
  sh "$checker" "$tmp/repository" v0.0.0-unsigned gitlab \
    >"$tmp/gitlab.out" 2>&1; then
  echo 'unsigned GitLab tag was accepted' >&2
  exit 1
fi
grep -F 'release tag is not SSH signed' "$tmp/gitlab.out" >/dev/null

echo 'release tag provider-selection regression: OK'
