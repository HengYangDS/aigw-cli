#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-forge-sync.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-forge-sync.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

canonical="$tmp/canonical"
git -c core.fsmonitor=false init -q -b main "$canonical"
git -c core.fsmonitor=false -C "$canonical" config user.name 'AIGW Forge Sync Test'
git -c core.fsmonitor=false -C "$canonical" config user.email 'aigw-forge-sync@example.invalid'
printf 'base\n' > "$canonical/state.txt"
git -c core.fsmonitor=false -C "$canonical" add state.txt
git -c core.fsmonitor=false -C "$canonical" commit -qm base
git -c core.fsmonitor=false -C "$canonical" switch -qc feature
printf 'feature\n' > "$canonical/feature.txt"
git -c core.fsmonitor=false -C "$canonical" add feature.txt
git -c core.fsmonitor=false -C "$canonical" commit -qm feature
git -c core.fsmonitor=false -C "$canonical" switch -q main
printf 'current\n' >> "$canonical/state.txt"
git -c core.fsmonitor=false -C "$canonical" commit -qam current
git -c core.fsmonitor=false -C "$canonical" merge -q --no-ff -m merge feature
canonical_tip=$(git -c core.fsmonitor=false -C "$canonical" rev-parse main)
canonical_parent=$(git -c core.fsmonitor=false -C "$canonical" rev-parse main~1)
canonical_tree_count=$(git -c core.fsmonitor=false -C "$canonical" rev-list --count main)
test "$canonical_tree_count" -eq 4
test "$(git -c core.fsmonitor=false -C "$canonical" rev-list --parents -n 1 main | awk '{print NF}')" -eq 3

projection="$tmp/projection"
git -c core.fsmonitor=false clone -q --no-local "file://$canonical" "$projection"
git -c core.fsmonitor=false -C "$projection" config user.name 'AIGW Forge Sync Test'
git -c core.fsmonitor=false -C "$projection" config user.email 'aigw-forge-sync@example.invalid'
FILTER_BRANCH_SQUELCH_WARNING=1 git -c core.fsmonitor=false -C "$projection" filter-branch -f --env-filter '
  GIT_AUTHOR_NAME="Projected"
  GIT_AUTHOR_EMAIL="projected@example.invalid"
  GIT_COMMITTER_NAME="Projected"
  GIT_COMMITTER_EMAIL="projected@example.invalid"
' -- main >/dev/null 2>&1
git -c core.fsmonitor=false -C "$projection" for-each-ref --format='%(refname)' refs/original/ | while IFS= read -r ref; do
  git -c core.fsmonitor=false -C "$projection" update-ref -d "$ref"
done
projected_tip=$(git -c core.fsmonitor=false -C "$projection" rev-parse main)
projected_parent=$(git -c core.fsmonitor=false -C "$projection" rev-parse main~1)
projected_parent_count=$(git -c core.fsmonitor=false -C "$projection" rev-list --count "$projected_parent")

git -c core.fsmonitor=false -C "$canonical" update-ref refs/remotes/gitlab/main "$canonical_tip"
git -c core.fsmonitor=false -C "$canonical" fetch -q "$projection" main
test "$(git -c core.fsmonitor=false -C "$canonical" rev-parse FETCH_HEAD)" = "$projected_tip"
git -c core.fsmonitor=false -C "$canonical" update-ref refs/remotes/github/main FETCH_HEAD

run_checker() {
  (
    cd "$canonical"
    GIT_CONFIG_GLOBAL=/dev/null GIT_SSH_COMMAND=false sh "$checker" "$@"
  )
}

expect_failure() {
  name=$1
  expected=$2
  shift 2
  output="$tmp/failure-output"
  if "$@" >"$output" 2>&1; then
    echo "forge sync checker accepted $name" >&2
    exit 1
  fi
  if ! grep -Fq "$expected" "$output"; then
    echo "forge sync checker reported an unexpected failure for $name" >&2
    cat "$output" >&2
    exit 1
  fi
}

output=$(run_checker --canonical main \
  --peer gitlab:refs/remotes/gitlab/main:commit \
  --peer github:refs/remotes/github/main:tree)
printf '%s\n' "$output" | grep -Fq "gitlab (refs/remotes/gitlab/main@$canonical_tip, commit) OK"
printf '%s\n' "$output" | grep -Fq "github (refs/remotes/github/main@$projected_tip, tree) OK"

printf 'dirty\n' >> "$canonical/state.txt"
run_checker --canonical main \
  --peer gitlab:refs/remotes/gitlab/main:commit \
  --peer github:refs/remotes/github/main:tree >/dev/null
git -c core.fsmonitor=false -C "$canonical" restore state.txt

git -c core.fsmonitor=false -C "$canonical" update-ref refs/remotes/gitlab/main "$canonical_parent"
expect_failure 'a lagging commit peer' 'does not exactly match canonical' \
  run_checker --canonical main --peer gitlab:refs/remotes/gitlab/main:commit

git -c core.fsmonitor=false -C "$canonical" switch -qc peer-divergence main
printf 'peer\n' > "$canonical/peer.txt"
git -c core.fsmonitor=false -C "$canonical" add peer.txt
git -c core.fsmonitor=false -C "$canonical" commit -qm peer
divergent_tip=$(git -c core.fsmonitor=false -C "$canonical" rev-parse HEAD)
git -c core.fsmonitor=false -C "$canonical" switch -q main
git -c core.fsmonitor=false -C "$canonical" update-ref refs/remotes/gitlab/main "$divergent_tip"
expect_failure 'a divergent commit peer' 'does not exactly match canonical' \
  run_checker --canonical main --peer gitlab:refs/remotes/gitlab/main:commit
git -c core.fsmonitor=false -C "$canonical" update-ref refs/remotes/gitlab/main "$canonical_tip"

git -c core.fsmonitor=false -C "$canonical" update-ref refs/remotes/github/main "$projected_parent"
expect_failure 'a shortened tree peer' "expected $canonical_tree_count entries, found $projected_parent_count" \
  run_checker --canonical main --peer github:refs/remotes/github/main:tree

git -c core.fsmonitor=false -C "$projection" switch -qc tree-divergence main
printf 'different\n' >> "$projection/state.txt"
git -c core.fsmonitor=false -C "$projection" commit -qam 'different tree' --amend
divergent_projection=$(git -c core.fsmonitor=false -C "$projection" rev-parse HEAD)
git -c core.fsmonitor=false -C "$canonical" fetch -q "$projection" tree-divergence
test "$(git -c core.fsmonitor=false -C "$canonical" rev-parse FETCH_HEAD)" = "$divergent_projection"
git -c core.fsmonitor=false -C "$canonical" update-ref refs/remotes/github/main FETCH_HEAD
expect_failure 'a divergent tree peer' "at position $canonical_tree_count" \
  run_checker --canonical main --peer github:refs/remotes/github/main:tree

expect_failure 'a missing peer ref' 'peer missing is unavailable' \
  run_checker --canonical main --peer missing:refs/remotes/missing/main:commit
expect_failure 'an invalid peer mode' 'peer gitlab has invalid mode' \
  run_checker --canonical main --peer gitlab:refs/remotes/gitlab/main:invalid
expect_failure 'a malformed peer specification' 'peer specification must include a name, ref, and mode' \
  run_checker --canonical main --peer :refs/remotes/gitlab/main:commit
expect_failure 'an unavailable canonical branch' 'canonical ref is unavailable or is not a local branch' \
  run_checker --canonical missing --peer gitlab:refs/remotes/gitlab/main:commit

echo 'forge synchronization contract: OK'
