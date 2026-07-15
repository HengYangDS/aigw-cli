#!/bin/sh
# Synchronize an explicitly selected ref across two equal Git forges without
# manufacturing commits, force-pushing, or deleting remote refs.
set -eu

usage() {
  cat >&2 <<'USAGE'
usage: forge-peer-sync.sh [--check|--sync] [--branch <name>|--tag <name>] [--gitlab-remote <name>] [--github-remote <name>]

--check (default) fetches both peer refs and reports convergence.
--sync fast-forward pushes the exact local ref to every reachable peer that is
behind. It refuses divergent peers, missing local refs, dirty worktrees,
force-pushes, ref deletion, and synthetic snapshot commits.
USAGE
  exit 2
}

mode=check
kind=branch
ref=main
gitlab_remote=${AIGW_GITLAB_REMOTE:-origin}
github_remote=${AIGW_GITHUB_REMOTE:-github}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --check) mode=check ;;
    --sync) mode=sync ;;
    --branch) kind=branch; ref=${2:?missing branch name}; shift ;;
    --tag) kind=tag; ref=${2:?missing tag name}; shift ;;
    --gitlab-remote) gitlab_remote=${2:?missing GitLab remote}; shift ;;
    --github-remote) github_remote=${2:?missing GitHub remote}; shift ;;
    -h|--help) usage ;;
    *) usage ;;
  esac
  shift
done

case "$ref" in ''|*' '*|*..*|*'~'*|*'^'*|*':'*|*'?'*|*'['*|*'\\'*) echo "invalid ref name: $ref" >&2; exit 2 ;; esac

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "run inside a Git worktree" >&2; exit 2; }
if [ "$mode" = sync ]; then
  git diff --quiet && git diff --cached --quiet || { echo "refusing sync with a dirty worktree" >&2; exit 2; }
fi

remote_url() { git remote get-url "$1" 2>/dev/null || return 1; }
validate_remote() {
  remote=$1 provider=$2
  url=$(remote_url "$remote") || { echo "$provider remote is not configured: $remote" >&2; return 1; }
  if [ "${AIGW_FORGE_PEER_SKIP_PROVIDER_URL_CHECK:-}" = "1" ]; then
    return 0
  fi
  case "$provider" in
    gitlab) markers=${AIGW_FORGE_PEER_GITLAB_URL_MARKERS:-'gitlab 192.168.64.101'} ;;
    github) markers=${AIGW_FORGE_PEER_GITHUB_URL_MARKERS:-'github.com'} ;;
    *) echo "unsupported forge provider: $provider" >&2; return 1 ;;
  esac
  for marker in $markers; do
    case "$url" in *"$marker"*) return 0 ;; esac
  done
  echo "$remote does not match a configured $provider forge URL marker" >&2
  return 1
}

local_ref() {
  case "$kind" in
    branch) printf 'refs/heads/%s' "$ref" ;;
    tag) printf 'refs/tags/%s' "$ref" ;;
  esac
}
remote_tracking_ref() {
  remote=$1
  case "$kind" in
    branch) printf 'refs/remotes/%s/%s' "$remote" "$ref" ;;
    tag) printf 'refs/tags/%s' "$ref" ;;
  esac
}
fetch_ref() {
  remote=$1
  case "$kind" in
    branch) git fetch --no-tags "$remote" "refs/heads/$ref:refs/remotes/$remote/$ref" ;;
    tag) git fetch --no-tags "$remote" "refs/tags/$ref:refs/tags/$ref" ;;
  esac
}

local=$(local_ref)
git rev-parse --verify -q "$local^{commit}" >/dev/null || { echo "local $kind does not resolve to a commit: $ref" >&2; exit 2; }
local_oid=$(git rev-parse "$local^{commit}")
reachable=''
for pair in "$gitlab_remote:gitlab" "$github_remote:github"; do
  remote=${pair%%:*}; provider=${pair#*:}
  if ! validate_remote "$remote" "$provider"; then
    continue
  fi
  if fetch_ref "$remote" >/dev/null 2>&1; then
    tracking=$(remote_tracking_ref "$remote")
    if git rev-parse --verify -q "$tracking^{commit}" >/dev/null; then
      oid=$(git rev-parse "$tracking^{commit}")
      printf '%s  %-7s %s\n' "$provider" "reachable" "$oid"
      reachable="$reachable $remote:$provider:$oid"
    else
      printf '%s  %-7s ref missing\n' "$provider" "reachable"
      reachable="$reachable $remote:$provider:missing"
    fi
  else
    printf '%s  %-7s unavailable\n' "$provider" "unavailable"
  fi
done

[ -n "$reachable" ] || { echo "no configured forge is reachable" >&2; exit 1; }

for item in $reachable; do
  remote=${item%%:*}; remainder=${item#*:}; provider=${remainder%%:*}; oid=${remainder#*:}
  [ "$oid" = missing ] && continue
  if ! git merge-base --is-ancestor "$oid" "$local_oid" && ! git merge-base --is-ancestor "$local_oid" "$oid"; then
    echo "$provider ref diverges from local $kind $ref; resolve manually without force push" >&2
    exit 1
  fi
done

if [ "$mode" = check ]; then
  for item in $reachable; do
    remainder=${item#*:}; provider=${remainder%%:*}; oid=${remainder#*:}
    if [ "$oid" = "$local_oid" ]; then
      printf '%s  %-7s converged\n' "$provider" "status"
    elif [ "$oid" = missing ]; then
      printf '%s  %-7s missing\n' "$provider" "status"
    else
      printf '%s  %-7s behind\n' "$provider" "status"
    fi
  done
  exit 0
fi

for item in $reachable; do
  remote=${item%%:*}; remainder=${item#*:}; provider=${remainder%%:*}; oid=${remainder#*:}
  if [ "$oid" = "$local_oid" ]; then
    printf '%s  %-7s already converged\n' "$provider" "sync"
    continue
  fi
  case "$kind" in
    branch) refspec="$local:refs/heads/$ref" ;;
    tag) refspec="$local:refs/tags/$ref" ;;
  esac
  git push "$remote" "$refspec"
  printf '%s  %-7s pushed %s\n' "$provider" "sync" "$local_oid"
done
