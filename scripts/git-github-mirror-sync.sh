#!/bin/sh
# Synchronize canonical GitLab refs to the independent GitHub repository.
# The GitHub projection keeps source history and trees, but rewrites GitLab
# author metadata to the GitHub identity before publishing.
set -eu

remote=${AIGW_GITHUB_MIRROR_REMOTE:-github-mirror}
branch=${AIGW_GITHUB_MIRROR_BRANCH:-main}
github_name=${AIGW_GITHUB_AUTHOR_NAME:-HengYang}
github_email=${AIGW_GITHUB_AUTHOR_EMAIL:-hengyang.2003@tsinghua.org.cn}

case "$github_email" in
  hengyang.2003@tsinghua.org.cn) ;;
  *) echo "GitHub mirror identity must be hengyang.2003@tsinghua.org.cn" >&2; exit 2 ;;
esac
case "$github_name" in
  HengYang) ;;
  *) echo "GitHub mirror author name must be HengYang" >&2; exit 2 ;;
esac

git rev-parse --is-inside-work-tree >/dev/null
git diff --quiet && git diff --cached --quiet || {
  echo "refusing GitHub projection sync with a dirty worktree" >&2
  exit 2
}
remote_url=$(git remote get-url "$remote")
case "$remote_url" in
  *github.com*) ;;
  *) echo "mirror remote $remote is not a GitHub remote" >&2; exit 2 ;;
esac

canonical=$(git rev-parse "$branch")
tree=$(git rev-parse "$canonical^{tree}")
workspace=$(mktemp -d "${TMPDIR:-/tmp}/aigw-github-projection.XXXXXX")
cleanup() { rm -rf "$workspace"; }
trap cleanup EXIT HUP INT TERM

git clone --quiet --no-local . "$workspace/repository"
projection="$workspace/repository"
git -C "$projection" remote remove origin 2>/dev/null || true
FILTER_BRANCH_SQUELCH_WARNING=1 git -C "$projection" filter-branch -f --tag-name-filter cat \
  --env-filter '
    GIT_AUTHOR_NAME="HengYang"
    GIT_AUTHOR_EMAIL="hengyang.2003@tsinghua.org.cn"
    GIT_COMMITTER_NAME="HengYang"
    GIT_COMMITTER_EMAIL="hengyang.2003@tsinghua.org.cn"
    GIT_TAGGER_NAME="HengYang"
    GIT_TAGGER_EMAIL="hengyang.2003@tsinghua.org.cn"
  ' -- --all >/dev/null 2>&1

git -C "$projection" for-each-ref --format='%(refname)' refs/original/ | while IFS= read -r ref; do
  git -C "$projection" update-ref -d "$ref"
done
if git -C "$projection" log --all --format='%ae%n%ce' | grep -Fv -x "$github_email" | grep -q .; then
  echo "GitHub projection retains a non-GitHub commit identity" >&2
  exit 1
fi
[ "$(git -C "$projection" rev-parse "$branch^{tree}")" = "$tree" ] || {
  echo "GitHub projection tree differs from canonical $branch" >&2
  exit 1
}

# Release tags are re-signed with the GitHub identity and preserve their
# canonical tagger timestamps so Changelog chronology remains stable.
for tag in $(git -C "$projection" tag --list 'v[0-9]*'); do
  target=$(git -C "$projection" rev-parse "$tag^{}")
  tagger_date=$(git for-each-ref "refs/tags/$tag" --format='%(taggerdate:iso8601)' | head -n 1)
  git -C "$projection" tag -d "$tag" >/dev/null
  GIT_COMMITTER_NAME="$github_name" GIT_COMMITTER_EMAIL="$github_email" GIT_COMMITTER_DATE="$tagger_date" \
    git -C "$projection" -c user.name="$github_name" -c user.email="$github_email" -c user.useConfigOnly=true \
      -c gpg.format=ssh -c user.signingkey="${AIGW_GIT_SIGNING_KEY:-$HOME/.ssh/id_ed25519_signing_yheng_20260711.pub}" \
      tag -s -a "$tag" -m "AIGW $tag GitHub provenance" "$target"
done

git -C "$projection" remote add github "$remote_url"
parent=$(git ls-remote --heads github "$branch" | awk 'NR==1 {print $1}')
[ -n "$parent" ] || { echo "GitHub branch $branch is missing" >&2; exit 1; }
projected=$(git -C "$projection" rev-parse "$branch")

# `--force-with-lease` makes concurrent GitHub mirror changes fail closed.
if ! git -C "$projection" -c user.name="$github_name" -c user.email="$github_email" -c user.useConfigOnly=true \
  push --force-with-lease="refs/heads/$branch:$parent" github "refs/heads/$branch:refs/heads/$branch" "refs/tags/*:refs/tags/*"; then
  cat >&2 <<'MSG'
GitHub mirror publication was rejected by GitHub email privacy protection.
Open https://github.com/settings/emails and make hengyang.2003@tsinghua.org.cn
public, or disable only "Block command line pushes that expose my email" after
confirming the address is intentional. Keep GitLab identity protection enabled.
MSG
  exit 1
fi
git update-ref "refs/remotes/$remote/$branch" "$projected" "$parent"
printf 'GitHub projection synchronized: %s\n' "$projected"
