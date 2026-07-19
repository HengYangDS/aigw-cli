#!/bin/sh
# Verify source-branch closeout without conflating canonical and projected IDs.
set -eu

usage() {
  cat >&2 <<'USAGE'
usage: check-branch-closeout.sh --source <commit-or-ref> [--canonical <ref>] [--peer <name:ref:mode>]...

Modes are `commit` for a peer that preserves commit identity and `tree` for an
identity-rewriting projection. A tree peer passes only when it preserves the
canonical branch's complete ordered source-tree history.
USAGE
  exit 2
}

source_ref=
canonical=main
peers=
while test "$#" -gt 0; do
  case "$1" in
    --source) source_ref=${2:?missing source ref}; shift ;;
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

test -n "$source_ref" || usage
test -n "$peers" || usage

git -c core.fsmonitor=false rev-parse --verify -q "$source_ref^{commit}" >/dev/null || {
  echo "source ref is unavailable: $source_ref" >&2
  exit 2
}
git -c core.fsmonitor=false rev-parse --verify -q "$canonical^{commit}" >/dev/null || {
  echo "canonical ref is unavailable: $canonical" >&2
  exit 2
}

if ! git -c core.fsmonitor=false merge-base --is-ancestor "$source_ref" "$canonical"; then
  echo "canonical ref does not contain source tip: $canonical <- $source_ref" >&2
  exit 1
fi

canonical_trees=$(mktemp "${TMPDIR:-/tmp}/aigw-closeout-canonical-trees.XXXXXX")
peer_trees=$(mktemp "${TMPDIR:-/tmp}/aigw-closeout-peer-trees.XXXXXX")
cleanup() { rm -f "$canonical_trees" "$peer_trees"; }
trap cleanup EXIT HUP INT TERM

git -c core.fsmonitor=false log --reverse --topo-order --format=%T "$canonical" > "$canonical_trees"

printf '%s\n' "$peers" | while IFS=: read -r name peer mode; do
  test -n "$name" || continue
  case "$mode" in
    commit|tree) ;;
    *) echo "peer $name has invalid mode: $mode" >&2; exit 2 ;;
  esac
  git -c core.fsmonitor=false rev-parse --verify -q "$peer^{commit}" >/dev/null || {
    echo "peer $name is unavailable: $peer" >&2
    exit 2
  }
  case "$mode" in
    commit)
      if ! git -c core.fsmonitor=false merge-base --is-ancestor "$source_ref" "$peer"; then
        echo "peer $name does not contain source tip: $peer <- $source_ref" >&2
        exit 1
      fi
      ;;
    tree)
      git -c core.fsmonitor=false log --reverse --topo-order --format=%T "$peer" > "$peer_trees"
      python3 - "$canonical_trees" "$peer_trees" "$name" <<'PYTHON'
from pathlib import Path
import sys

canonical = Path(sys.argv[1]).read_text(encoding="utf-8").splitlines()
peer = Path(sys.argv[2]).read_text(encoding="utf-8").splitlines()
name = sys.argv[3]
if canonical == peer:
    raise SystemExit(0)
if len(canonical) != len(peer):
    raise SystemExit(
        f"peer {name} does not preserve canonical ordered source-tree history: "
        f"expected {len(canonical)} entries, found {len(peer)}"
    )
for position, (expected, actual) in enumerate(zip(canonical, peer), 1):
    if expected != actual:
        raise SystemExit(
            f"peer {name} does not preserve canonical ordered source-tree history "
            f"at position {position}: expected {expected}, found {actual}"
        )
raise SystemExit(f"peer {name} does not preserve canonical ordered source-tree history")
PYTHON
      ;;
  esac
  printf 'branch closeout peer: %s (%s) OK\n' "$name" "$mode"
done

printf 'branch closeout canonical: %s contains %s\n' "$canonical" "$source_ref"
