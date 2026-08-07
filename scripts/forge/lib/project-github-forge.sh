#!/bin/sh
# Project one complete canonical GitLab graph into the GitHub identity domain.
set -eu

usage() {
  cat >&2 <<'USAGE'
usage: project-github-forge.sh [--branch <name>] [--github-remote <name>]

Projects one canonical branch into the GitHub peer repository with a complete
GitHub identity graph and trusted GitHub commit signatures.
Existing GitHub release tags must verify as GitHub provenance before the branch update. The
update must be a fast-forward; no tag ref is pushed and no rewrite escape exists.
USAGE
  exit 2
}

branch=main
github_remote=${AIGW_GITHUB_REMOTE:-github}
github_name=${AIGW_GITHUB_AUTHOR_NAME:-}
github_email=${AIGW_GITHUB_AUTHOR_EMAIL:-}
gitlab_email=${AIGW_GITLAB_AUTHOR_EMAIL:-}
script_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
github_allowed_signers=${AIGW_GITHUB_ALLOWED_SIGNERS:-}
gitlab_allowed_signers=${AIGW_GITLAB_ALLOWED_SIGNERS:-}
github_signing_key=${AIGW_GITHUB_SIGNING_KEY:-}
github_signing_program=${AIGW_GITHUB_SIGNING_PROGRAM:-}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --branch) branch=${2:?missing branch name}; shift ;;
    --github-remote) github_remote=${2:?missing GitHub remote}; shift ;;
    -h|--help) usage ;;
    *) usage ;;
  esac
  shift
done

case "$branch" in ''|*' '*|*..*|*'~'*|*'^'*|*':'*|*'?'*|*'['*|*'\'*) echo "invalid branch name: $branch" >&2; exit 2 ;; esac
[ -n "$github_name" ] || { echo "GitHub author name is required through AIGW_GITHUB_AUTHOR_NAME" >&2; exit 2; }
case "$github_email" in *@*.*) ;; *) echo "GitHub author email is required through AIGW_GITHUB_AUTHOR_EMAIL" >&2; exit 2 ;; esac
case "$gitlab_email" in *@*.*) ;; *) echo "canonical author email is required through AIGW_GITLAB_AUTHOR_EMAIL" >&2; exit 2 ;; esac
[ -f "$github_allowed_signers" ] || { echo "GitHub trust input is required through AIGW_GITHUB_ALLOWED_SIGNERS" >&2; exit 2; }
[ -f "$gitlab_allowed_signers" ] || { echo "GitLab trust input is required through AIGW_GITLAB_ALLOWED_SIGNERS" >&2; exit 2; }

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "run inside a Git worktree" >&2; exit 2; }
git diff --quiet && git diff --cached --quiet || { echo "refusing GitHub projection with a dirty canonical worktree" >&2; exit 2; }

canonical_root=$(git rev-parse --show-toplevel)
canonical=$(git rev-parse "refs/heads/$branch")
github_url=$(git config --local --get "remote.$github_remote.url" 2>/dev/null) || {
  echo "GitHub remote is not configured: $github_remote" >&2
  exit 2
}
case "$github_url" in *github.com*|file://*) ;; *) echo "$github_remote is not a GitHub remote" >&2; exit 2 ;; esac

if [ -z "$github_signing_key" ]; then
  github_signing_key=$(git config --get aigw.githubSigningKey 2>/dev/null || true)
fi
[ -n "$github_signing_key" ] || {
  echo 'GitHub signing key is required through AIGW_GITHUB_SIGNING_KEY or aigw.githubSigningKey' >&2
  exit 2
}
if [ -z "$github_signing_program" ]; then
  github_signing_program=$(git config --get aigw.githubSigningProgram 2>/dev/null || true)
fi
github_signing_program=${github_signing_program:-ssh-keygen}
case "$github_signing_program" in
  */*) [ -x "$github_signing_program" ] || { echo "GitHub signing program is not executable: $github_signing_program" >&2; exit 2; } ;;
  *) command -v "$github_signing_program" >/dev/null 2>&1 || { echo "GitHub signing program is unavailable: $github_signing_program" >&2; exit 2; } ;;
esac

AIGW_GITLAB_ALLOWED_SIGNERS="$gitlab_allowed_signers" \
  AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
  sh "$script_root/scripts/checks/forge/check-commit-provenance.sh" "$canonical_root" gitlab >/dev/null

workspace=$(mktemp -d "${TMPDIR:-/tmp}/aigw-github-projection.XXXXXX")
cleanup() { rm -rf "$workspace"; }
trap cleanup EXIT HUP INT TERM
projection="$workspace/repository.git"

go -C "$script_root" run ./tools/historyreplay \
  --source "$canonical_root" \
  --revision "$canonical" \
  --output "$projection" \
  --ref "refs/heads/$branch" \
  --actor-name "$github_name" \
  --actor-email "$github_email" \
  --signing-key "$github_signing_key" \
  --signing-program "$github_signing_program" \
  --allowed-signers "$github_allowed_signers" >/dev/null
projected=$(git -C "$projection" rev-parse "refs/heads/$branch")

git_transport() { GIT_CONFIG_GLOBAL=/dev/null git "$@"; }
git -C "$projection" remote add github "$github_url"
remote_tip=$(git_transport -C "$projection" ls-remote --heads github "refs/heads/$branch" | awk 'NR==1 {print $1}')
if [ -n "$remote_tip" ]; then
  git_transport -C "$projection" fetch --quiet --no-tags github "refs/heads/$branch:refs/remotes/github/$branch"
  git -C "$projection" merge-base --is-ancestor "$remote_tip" "$projected" || {
    echo "GitHub branch diverges from the complete canonical identity projection; resolve manually" >&2
    exit 1
  }

  git -C "$projection" symbolic-ref HEAD "refs/remotes/github/$branch"
  AIGW_GITHUB_ALLOWED_SIGNERS="$github_allowed_signers" \
    AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
    sh "$script_root/scripts/checks/forge/check-commit-provenance.sh" "$projection" github >/dev/null
  git -C "$projection" symbolic-ref HEAD "refs/heads/$branch"
fi

remote_tags="$workspace/remote-tags"
git_transport -C "$projection" ls-remote --tags github 'v[0-9]*' > "$remote_tags"
canonical_trees="$workspace/canonical-trees"
git -C "$canonical_root" log "$canonical" --format=%T | LC_ALL=C sort -u > "$canonical_trees"
for tag in $(awk '$2 !~ /\^\{\}$/ {sub("refs/tags/", "", $2); print $2}' "$remote_tags" | LC_ALL=C sort -u); do
  git_transport -C "$projection" fetch --quiet --no-tags github "refs/tags/$tag:refs/tags/github/$tag"
  [ "$(git -C "$projection" cat-file -t "github/$tag")" = tag ] || {
    echo "GitHub release tag must be annotated: $tag" >&2
    exit 1
  }
  tag_tree=$(git -C "$projection" rev-parse "github/$tag^{}^{tree}")
  grep -F -x "$tag_tree" "$canonical_trees" >/dev/null || continue
  if git -C "$canonical_root" tag --merged "$canonical" --list "$tag" | grep -Fxq "$tag"; then
    git -C "$canonical_root" -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
      -c gpg.ssh.allowedSignersFile="$gitlab_allowed_signers" verify-tag "$tag" >/dev/null 2>&1 || {
      echo "canonical GitLab tag does not verify: $tag" >&2
      exit 1
    }
  fi
  if ! git -C "$projection" -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
    -c gpg.ssh.allowedSignersFile="$github_allowed_signers" verify-tag "github/$tag" >/dev/null 2>&1; then
    echo "GitHub provenance tag does not verify: $tag" >&2
    exit 1
  fi
done

git_transport -C "$projection" \
  -c user.name="$github_name" \
  -c user.email="$github_email" \
  -c user.useConfigOnly=true \
  push --quiet github "refs/heads/$branch:refs/heads/$branch"
printf 'GitHub provider projection synchronized: %s\n' "$projected"
