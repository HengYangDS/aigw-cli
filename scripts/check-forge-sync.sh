#!/bin/sh
# Verify already-refreshed forge refs without network access or ref writes.
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

usage() {
  cat >&2 <<'USAGE'
usage: check-forge-sync.sh [--canonical <local-branch>] --peer <name:ref:mode>...

Modes are `commit` for a peer that must exactly preserve canonical commit
identity and `tree` for an identity-rewriting projection that must preserve the
canonical branch's complete ordered source-tree history. This command never
fetches or writes refs; callers must refresh every peer ref before invoking it.
USAGE
  exit 2
}

canonical=main
peers=
while test "$#" -gt 0; do
  case "$1" in
    --canonical) canonical=${2:?missing canonical ref}; shift ;;
    --peer)
      peers="${peers}${peers:+
}${2:?missing peer specification}"
      shift
      ;;
    -h|--help) usage ;;
    *) usage ;;
  esac
  shift
done

test -n "$peers" || usage

normalize_local_branch() {
  raw=$1
  normalized=$(git -c core.fsmonitor=false rev-parse --symbolic-full-name --verify "$raw" 2>/dev/null) || normalized=
  case "$normalized" in
    refs/heads/*) ;;
    *)
      echo "canonical ref is unavailable or is not a local branch: $raw" >&2
      return 2
      ;;
  esac
  git -c core.fsmonitor=false rev-parse --verify -q "$normalized^{commit}" >/dev/null || {
    echo "canonical ref is unavailable: $raw" >&2
    return 2
  }
  printf '%s\n' "$normalized"
}

canonical=$(normalize_local_branch "$canonical")
canonical_commit=$(git -c core.fsmonitor=false rev-parse "$canonical^{commit}")
canonical_trees=$(mktemp "${TMPDIR:-/tmp}/aigw-forge-sync-canonical-trees.XXXXXX")
peer_trees=$(mktemp "${TMPDIR:-/tmp}/aigw-forge-sync-peer-trees.XXXXXX")
cleanup() { rm -f "$canonical_trees" "$peer_trees"; }
trap cleanup EXIT HUP INT TERM

git -c core.fsmonitor=false log --reverse --topo-order --format=%T "$canonical" > "$canonical_trees"

printf '%s\n' "$peers" | while IFS=: read -r name peer mode; do
  if test -z "$name" || test -z "$peer" || test -z "$mode"; then
    echo "peer specification must include a name, ref, and mode" >&2
    exit 2
  fi
  case "$mode" in
    commit|tree) ;;
    *) echo "peer $name has invalid mode: $mode" >&2; exit 2 ;;
  esac
  peer_commit=$(git -c core.fsmonitor=false rev-parse --verify -q "$peer^{commit}") || {
    echo "peer $name is unavailable: $peer" >&2
    exit 2
  }
  case "$mode" in
    commit)
      if test "$peer_commit" != "$canonical_commit"; then
        echo "peer $name does not exactly match canonical $canonical: $peer@$peer_commit, expected $canonical_commit" >&2
        exit 1
      fi
      ;;
    tree)
      git -c core.fsmonitor=false log --reverse --topo-order --format=%T "$peer" > "$peer_trees"
      python3 "$script_dir/compare-ordered-trees.py" "$canonical_trees" "$peer_trees" "$name"
      ;;
  esac
  printf 'forge sync peer: %s (%s@%s, %s) OK\n' "$name" "$peer" "$peer_commit" "$mode"
done

printf 'forge sync canonical: %s@%s\n' "$canonical" "$canonical_commit"
