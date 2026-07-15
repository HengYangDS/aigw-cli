#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
script="$root/scripts/forge-peer-sync.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

make_remote() {
  git init --bare -q "$1"
}
make_remote "$tmp/gitlab.git"
make_remote "$tmp/github.git"
git init -q "$tmp/repo"
(
  cd "$tmp/repo"
  git config user.name Test
  git config user.email test@example.test
  printf 'one\n' > file
  git add file
  git commit -qm initial
  git branch -M main
  git remote add gitlab "$tmp/gitlab.git"
  git remote add github "$tmp/github.git"
  git -c core.hooksPath=/dev/null push -q gitlab main
  git -c core.hooksPath=/dev/null push -q github main
  printf 'two\n' >> file
  git commit -am second -q
  AIGW_FORGE_PEER_SKIP_PROVIDER_URL_CHECK=1 "$script" --check --branch main --gitlab-remote gitlab --github-remote github > "$tmp/check"
  grep -q 'gitlab  status  behind' "$tmp/check"
  grep -q 'github  status  behind' "$tmp/check"
  AIGW_FORGE_PEER_SKIP_PROVIDER_URL_CHECK=1 GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null "$script" --sync --branch main --gitlab-remote gitlab --github-remote github > "$tmp/sync"
  local_oid=$(git rev-parse main)
  test "$(git ls-remote gitlab refs/heads/main | awk '{print $1}')" = "$local_oid"
  test "$(git ls-remote github refs/heads/main | awk '{print $1}')" = "$local_oid"
  test "$(git rev-list --count main)" = 2
  printf 'dirty\n' >> file
  if AIGW_FORGE_PEER_SKIP_PROVIDER_URL_CHECK=1 GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null "$script" --sync --branch main --gitlab-remote gitlab --github-remote github >/dev/null 2>&1; then
    echo "sync accepted a dirty worktree" >&2
    exit 1
  fi
)

echo "equal forge synchronization contract: OK"
