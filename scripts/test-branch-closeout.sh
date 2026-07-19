#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-branch-closeout.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-branch-closeout.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

source="$tmp/source"
git -c core.fsmonitor=false init -q -b main "$source"
git -c core.fsmonitor=false -C "$source" config user.name 'AIGW Closeout Test'
git -c core.fsmonitor=false -C "$source" config user.email 'aigw-closeout@example.invalid'
printf 'base\n' > "$source/state.txt"
git -c core.fsmonitor=false -C "$source" add state.txt
git -c core.fsmonitor=false -C "$source" commit -qm base
git -c core.fsmonitor=false -C "$source" switch -qc work/source
printf 'source\n' >> "$source/state.txt"
git -c core.fsmonitor=false -C "$source" commit -qam source
git -c core.fsmonitor=false -C "$source" switch -q main
git -c core.fsmonitor=false -C "$source" merge -q --ff-only work/source

projection="$tmp/projection"
git -c core.fsmonitor=false clone -q --no-local "file://$source" "$projection"
git -c core.fsmonitor=false -C "$projection" config user.name 'AIGW Closeout Test'
git -c core.fsmonitor=false -C "$projection" config user.email 'aigw-closeout@example.invalid'
FILTER_BRANCH_SQUELCH_WARNING=1 git -c core.fsmonitor=false -C "$projection" filter-branch -f --env-filter '
  GIT_AUTHOR_NAME="Projected"
  GIT_AUTHOR_EMAIL="projected@example.invalid"
  GIT_COMMITTER_NAME="Projected"
  GIT_COMMITTER_EMAIL="projected@example.invalid"
' -- main >/dev/null 2>&1
git -c core.fsmonitor=false -C "$projection" for-each-ref --format='%(refname)' refs/original/ | while IFS= read -r ref; do
  git -c core.fsmonitor=false -C "$projection" update-ref -d "$ref"
done
git -c core.fsmonitor=false -C "$source" fetch -q "$projection" main
git -c core.fsmonitor=false -C "$source" update-ref \
  refs/closeout/github-projection-main FETCH_HEAD

(
  cd "$source"
  sh "$checker" --source work/source --canonical main \
    --peer canonical:main:commit \
    --peer github:refs/closeout/github-projection-main:tree
)

failures=0
if (
  cd "$source"
  sh "$checker" --source main --canonical main \
    --peer canonical:main:commit
) >/dev/null 2>&1; then
  echo 'branch closeout checker accepted the canonical branch as its own source' >&2
  failures=$((failures + 1))
fi

source_worktree="$tmp/source-worktree"
git -c core.fsmonitor=false -C "$source" worktree add -q "$source_worktree" work/source
printf 'tracked dirt\n' >> "$source_worktree/state.txt"
if (
  cd "$source"
  sh "$checker" --source refs/heads/work/source --canonical refs/heads/main \
    --peer canonical:main:commit
) >/dev/null 2>&1; then
  echo 'branch closeout checker accepted tracked dirt in the source worktree' >&2
  failures=$((failures + 1))
fi
git -c core.fsmonitor=false -C "$source_worktree" restore state.txt

printf 'untracked dirt\n' > "$source_worktree/untracked.txt"
if (
  cd "$source"
  sh "$checker" --source work/source --canonical main \
    --peer canonical:main:commit
) >/dev/null 2>&1; then
  echo 'branch closeout checker accepted untracked dirt in the source worktree' >&2
  failures=$((failures + 1))
fi
rm -f "$source_worktree/untracked.txt"

unavailable_worktree="$tmp/source-worktree-unavailable"
mv "$source_worktree" "$unavailable_worktree"
if (
  cd "$source"
  sh "$checker" --source work/source --canonical main \
    --peer canonical:main:commit
) >/dev/null 2>&1; then
  echo 'branch closeout checker accepted an unavailable source worktree' >&2
  failures=$((failures + 1))
fi
mv "$unavailable_worktree" "$source_worktree"

test "$failures" -eq 0 || exit 1

printf 'different\n' > "$projection/state.txt"
git -c core.fsmonitor=false -C "$projection" commit -qam divergence
git -c core.fsmonitor=false -C "$source" fetch -q "$projection" main
git -c core.fsmonitor=false -C "$source" update-ref \
  refs/closeout/github-projection-main FETCH_HEAD
if (
  cd "$source"
  sh "$checker" --source work/source --canonical main \
    --peer github:refs/closeout/github-projection-main:tree
) >/dev/null 2>&1; then
  echo 'branch closeout checker accepted a projected tree divergence' >&2
  exit 1
fi

if (
  cd "$source"
  sh "$checker" --source work/source --canonical main \
    --peer missing:refs/remotes/missing/main:commit
) >/dev/null 2>&1; then
  echo 'branch closeout checker accepted a missing required peer ref' >&2
  exit 1
fi

echo 'branch closeout provider-equivalence contract: OK'
