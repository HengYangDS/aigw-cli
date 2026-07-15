#!/bin/sh
# Synchronize canonical GitLab history to an independently stored GitHub
# repository. The projection is deliberately isolated: every history-rewrite
# command runs only in a detached file:// clone, never in this checkout.
set -eu

remote=${AIGW_GITHUB_MIRROR_REMOTE:-github-mirror}
branch=${AIGW_GITHUB_MIRROR_BRANCH:-main}
github_name=${AIGW_GITHUB_AUTHOR_NAME:-HengYang}
github_email=${AIGW_GITHUB_AUTHOR_EMAIL:-hengyang.2003@tsinghua.org.cn}
allowed_signers=${AIGW_GITHUB_ALLOWED_SIGNERS:-$HOME/.config/git/aigw-github-allowed-signers}

case "$github_email" in
  hengyang.2003@tsinghua.org.cn) ;;
  *) echo "GitHub projection identity must be hengyang.2003@tsinghua.org.cn" >&2; exit 2 ;;
esac
case "$github_name" in
  HengYang) ;;
  *) echo "GitHub projection author name must be HengYang" >&2; exit 2 ;;
esac

git rev-parse --is-inside-work-tree >/dev/null
git diff --quiet && git diff --cached --quiet || {
  echo "refusing GitHub projection sync with a dirty canonical worktree" >&2
  exit 2
}

canonical_dir=$(git rev-parse --path-format=absolute --git-common-dir)
canonical_root=$(git rev-parse --show-toplevel)
remote_configured_url=$(git config --get "remote.$remote.url" || true)
case "$remote_configured_url" in
  *github.com*) ;;
  *) echo "mirror remote $remote is not a GitHub remote" >&2; exit 2 ;;
esac
remote_url=$(git remote get-url "$remote")

canonical=$(git rev-parse "$branch")
tree=$(git rev-parse "$canonical^{tree}")
workspace=$(mktemp -d "${TMPDIR:-/tmp}/aigw-github-projection.XXXXXX")
cleanup() { rm -rf "$workspace"; }
trap cleanup EXIT HUP INT TERM

# `file://` forces a fresh object database. In particular, do not use a Git
# worktree, a local clone optimisation, alternates, or a shared clone here.
projection="$workspace/repository"
git clone --quiet --no-local "file://$canonical_root" "$projection"
projection_dir=$(git -C "$projection" rev-parse --path-format=absolute --git-common-dir)
[ "$projection_dir" != "$canonical_dir" ] || {
  echo "refusing projection: clone shares the canonical Git common directory" >&2
  exit 1
}
[ ! -e "$projection_dir/objects/info/alternates" ] || {
  echo "refusing projection: clone has object alternates" >&2
  exit 1
}
git -C "$projection" remote remove origin 2>/dev/null || true

# Restrict the rewrite to canonical release-history refs. Hidden refs (such as
# a CI remote-tracking ref) must not accidentally become a GitHub publication.
git -C "$projection" for-each-ref --format='delete %(refname)' refs/heads refs/tags | git -C "$projection" update-ref --stdin
git -C "$projection" branch --force "$branch" "$canonical"
git -C "$projection" checkout --quiet "$branch"
git for-each-ref --format='%(refname)' refs/tags/v'[0-9]*' | while IFS= read -r ref; do
  git -C "$projection" update-ref "$ref" "$(git rev-parse "$ref")"
done

FILTER_BRANCH_SQUELCH_WARNING=1 git -C "$projection" filter-branch -f \
  --env-filter '
    GIT_AUTHOR_NAME="HengYang"
    GIT_AUTHOR_EMAIL="hengyang.2003@tsinghua.org.cn"
    GIT_COMMITTER_NAME="HengYang"
    GIT_COMMITTER_EMAIL="hengyang.2003@tsinghua.org.cn"
    GIT_TAGGER_NAME="HengYang"
    GIT_TAGGER_EMAIL="hengyang.2003@tsinghua.org.cn"
  ' -- "$branch" --tags >/dev/null 2>&1

git -C "$projection" for-each-ref --format='%(refname)' refs/original/ | while IFS= read -r ref; do
  git -C "$projection" update-ref -d "$ref"
done
if git -C "$projection" log "$branch" --format='%ae%n%ce' | grep -Fv -x "$github_email" | grep -q .; then
  echo "GitHub projection retains a non-GitHub commit identity" >&2
  exit 1
fi
[ "$(git -C "$projection" rev-parse "$branch^{tree}")" = "$tree" ] || {
  echo "GitHub projection tree differs from canonical $branch" >&2
  exit 1
}

# The protected GitHub provenance tag series is immutable after publication.
# Existing tags must be bit-for-bit equal; an absent tag is created exactly
# once with GitHub provenance and the canonical tagger timestamp.
git -C "$projection" remote add github "$remote_url"
remote_branch=$(git -C "$projection" ls-remote --heads github "$branch" | awk 'NR==1 {print $1}')
[ -n "$remote_branch" ] || {
  echo "GitHub branch $branch is missing; bootstrap it through a reviewed initial projection" >&2
  exit 1
}
remote_tags="$workspace/remote-tags"
git -C "$projection" ls-remote --tags github 'v[0-9]*' > "$remote_tags"

# A GitHub projection is a provider-specific trust namespace. An already
# published provenance tag is authoritative and must verify under the GitHub
# signer; it is never regenerated or force-updated. New historical tags are
# deliberately not backfilled by sync: formal tags must be released through
# their own provider-specific release workflow.
for tag in $(git -C "$projection" tag --list 'v[0-9]*'); do
  remote_tag=$(awk -v "needle=refs/tags/$tag" '$2 == needle {print $1; exit}' "$remote_tags")
  [ -n "$remote_tag" ] || continue
  git -C "$projection" fetch --quiet github "refs/tags/$tag:refs/tags/github/$tag"
  git -C "$projection" -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen -c gpg.ssh.allowedSignersFile="$allowed_signers" verify-tag "github/$tag" >/dev/null 2>&1 || {
    echo "GitHub provenance tag $tag does not verify under the GitHub trust anchor" >&2
    exit 1
  }
done

projected=$(git -C "$projection" rev-parse "$branch")
# The projected branch has rewritten commit identities, so its object IDs can
# never be ancestry-compatible with the GitLab branch. Its previous GitHub
# tree must instead be an ancestor tree of the canonical source: GitHub may
# lag, but may not carry an independent source divergence.
remote_tip=$(git -C "$projection" ls-remote github "refs/heads/$branch" | awk 'NR==1 {print $1}')
git -C "$projection" fetch --quiet github "refs/heads/$branch:refs/remotes/github/$branch"
remote_source_tree=$(git -C "$projection" rev-parse "refs/remotes/github/$branch^{tree}")
if ! git -C "$projection" log "$canonical" --format='%T' | grep -F -x "$remote_source_tree" >/dev/null; then
  echo "GitHub branch $branch diverged from canonical source history; reconcile GitHub independently" >&2
  exit 1
fi

# A lease detects concurrent change after the tree-equivalence check. The
# history is projection-specific, so this controlled force-with-lease is the
# only permitted ref rewrite; it never rewrites tags.
if ! git -C "$projection" -c user.name="$github_name" -c user.email="$github_email" -c user.useConfigOnly=true \
  push --force-with-lease="refs/heads/$branch:$remote_tip" github "refs/heads/$branch:refs/heads/$branch"; then
  cat >&2 <<'MSG'
GitHub projection publication failed. The GitHub branch may have diverged, or
GitHub may have rejected the intended GitHub identity. Do not force-push.
Reconcile the independent GitHub repository and retry.
MSG
  exit 1
fi
printf 'GitHub projection synchronized: %s\n' "$projected"
