#!/bin/sh
# Project canonical GitLab history into the GitHub identity domain. Only an
# isolated clone is rewritten; canonical refs and provider-native release tags
# are never altered or pushed by this command.
set -eu

usage() {
  cat >&2 <<'USAGE'
usage: project-github-forge.sh [--branch <name>] [--github-remote <name>]

Projects one canonical branch into the GitHub peer repository with the GitHub
commit identity. Existing GitHub release tags must verify as GitHub provenance
before the branch update. The branch update is leased; no tag ref is pushed.
USAGE
  exit 2
}

branch=main
github_remote=${AIGW_GITHUB_REMOTE:-github}
github_name=${AIGW_GITHUB_AUTHOR_NAME:-HengYang}
github_email=${AIGW_GITHUB_AUTHOR_EMAIL:-hengyang.2003@tsinghua.org.cn}
release_directory=$(CDPATH= cd -- "$(dirname -- "$0")/../packaging/release" && pwd)
github_allowed_signers=${AIGW_GITHUB_ALLOWED_SIGNERS:-$release_directory/github-allowed-signers}
github_legacy_allowed_signers=${AIGW_GITHUB_LEGACY_ALLOWED_SIGNERS:-$release_directory/github-legacy-allowed-signers}
github_legacy_tags=${AIGW_GITHUB_LEGACY_TAGS:-$release_directory/github-legacy-tags.txt}
gitlab_allowed_signers=${AIGW_GITLAB_ALLOWED_SIGNERS:-$release_directory/gitlab-allowed-signers}

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
case "$github_name" in HengYang) ;; *) echo "GitHub author name must be HengYang" >&2; exit 2 ;; esac
case "$github_email" in hengyang.2003@tsinghua.org.cn) ;; *) echo "GitHub identity must be hengyang.2003@tsinghua.org.cn" >&2; exit 2 ;; esac

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "run inside a Git worktree" >&2; exit 2; }
git diff --quiet && git diff --cached --quiet || { echo "refusing GitHub projection with a dirty canonical worktree" >&2; exit 2; }

root=$(git rev-parse --show-toplevel)
canonical_common=$(git rev-parse --path-format=absolute --git-common-dir)
canonical=$(git rev-parse "refs/heads/$branch")
canonical_tree=$(git rev-parse "$canonical^{tree}")
# Keep the exact repository-local transport endpoint. `git remote get-url`
# expands user-global `url.*.insteadOf` rules, which can silently turn a
# configured SSH remote into HTTPS and change the caller's authentication path.
github_url=$(git config --local --get "remote.$github_remote.url" 2>/dev/null) || { echo "GitHub remote is not configured: $github_remote" >&2; exit 2; }
case "$github_url" in *github.com*|file://*) ;; *) echo "$github_remote is not a GitHub remote" >&2; exit 2 ;; esac

workspace=$(mktemp -d "${TMPDIR:-/tmp}/aigw-github-projection.XXXXXX")
cleanup() { rm -rf "$workspace"; }
trap cleanup EXIT HUP INT TERM
projection="$workspace/repository"
projection_source_ref="refs/aigw-projection/canonical"

# Provider projection is an explicit, self-contained transport operation. The
# user's global Git configuration remains untouched, but cannot rewrite its
# configured remote URL underneath this controlled operation.
git_transport() {
  GIT_CONFIG_GLOBAL=/dev/null git "$@"
}

# file:// forces a fresh object database. Linked worktrees, local clone
# optimisations, alternates, and shared object databases are explicitly barred.
git clone --quiet --no-local "file://$root" "$projection"
projection_common=$(git -C "$projection" rev-parse --path-format=absolute --git-common-dir)
[ "$projection_common" != "$canonical_common" ] || { echo "projection clone shares canonical Git common directory" >&2; exit 1; }
[ ! -e "$projection_common/objects/info/alternates" ] || { echo "projection clone has object alternates" >&2; exit 1; }
git -C "$projection" remote remove origin 2>/dev/null || true

# Limit rewrite inputs to the one canonical branch. Provider-native tags stay
# outside the rewritten namespace and are never copied or regenerated.  Keep
# the source commit reachable through a private temporary ref and detach HEAD
# before deleting heads: deleting the branch currently checked out by a clone
# otherwise turns every tracked file into a staged addition.
git -C "$projection" update-ref "$projection_source_ref" "$canonical"
git -C "$projection" checkout --detach --quiet "$projection_source_ref"
git -C "$projection" for-each-ref --format='delete %(refname)' refs/heads refs/tags | git -C "$projection" update-ref --stdin
git -C "$projection" branch --force "$branch" "$projection_source_ref"
git -C "$projection" checkout --quiet "$branch"
git -C "$projection" update-ref -d "$projection_source_ref"
test -z "$(git -C "$projection" status --porcelain)" || {
  echo "projection clone is not clean before identity rewrite" >&2
  exit 1
}
FILTER_BRANCH_SQUELCH_WARNING=1 git -C "$projection" filter-branch -f \
  --env-filter '
    GIT_AUTHOR_NAME="HengYang"
    GIT_AUTHOR_EMAIL="hengyang.2003@tsinghua.org.cn"
    GIT_COMMITTER_NAME="HengYang"
    GIT_COMMITTER_EMAIL="hengyang.2003@tsinghua.org.cn"
  ' -- "$branch" >/dev/null 2>&1
git -C "$projection" for-each-ref --format='%(refname)' refs/original/ | while IFS= read -r ref; do
  git -C "$projection" update-ref -d "$ref"
done

projected=$(git -C "$projection" rev-parse "refs/heads/$branch")
[ "$(git -C "$projection" rev-parse "$projected^{tree}")" = "$canonical_tree" ] || { echo "projected GitHub branch tree differs from canonical branch" >&2; exit 1; }
if git -C "$projection" log "$projected" --format='%ae%n%ce' | grep -Fv -x "$github_email" | grep -q .; then
  echo "projected GitHub history retains a non-GitHub identity" >&2
  exit 1
fi

git -C "$projection" remote add github "$github_url"
remote_tip=$(git_transport -C "$projection" ls-remote --heads github "refs/heads/$branch" | awk 'NR==1 {print $1}')
[ -n "$remote_tip" ] || { echo "GitHub branch is missing: $branch" >&2; exit 1; }
git_transport -C "$projection" fetch --quiet --no-tags github "refs/heads/$branch:refs/remotes/github/$branch"
remote_tree=$(git -C "$projection" rev-parse "refs/remotes/github/$branch^{tree}")
# The remote GitHub history is a rewritten identity projection. It may lag but
# its tree must be on the canonical branch's history; unrelated content is a
# divergence, not a force-push invitation.
if ! git -C "$projection" log "$canonical" --format='%T' | grep -F -x "$remote_tree" >/dev/null; then
  echo "GitHub branch tree diverges from canonical history; resolve manually" >&2
  exit 1
fi

# Every GitHub release tag whose source tree is represented by the selected
# canonical branch must verify under the GitHub trust policy before the branch
# projection is updated.  This covers both same-named GitLab/GitHub releases
# and GitHub-only releases created after an identity projection. Tags remain
# outside this branch-only projection; no tag is copied, regenerated, or
# pushed. Historical tags whose trees are absent from the selected branch do
# not block a current projection.
remote_tags="$workspace/remote-tags"
git_transport -C "$projection" ls-remote --tags github 'v[0-9]*' > "$remote_tags"
canonical_trees="$workspace/canonical-trees"
git -C "$root" log "$canonical" --format=%T | LC_ALL=C sort -u > "$canonical_trees"
for tag in $(awk '$2 !~ /\^\{\}$/ {sub("refs/tags/", "", $2); print $2}' "$remote_tags" | LC_ALL=C sort -u); do
  git_transport -C "$projection" fetch --quiet --no-tags github "refs/tags/$tag:refs/tags/github/$tag"
  if [ "$(git -C "$projection" cat-file -t "github/$tag")" != tag ]; then
    echo "GitHub release tag must be annotated: $tag" >&2
    exit 1
  fi
  tag_tree=$(git -C "$projection" rev-parse "github/$tag^{}^{tree}")
  grep -F -x "$tag_tree" "$canonical_trees" >/dev/null || continue
  # When the selected canonical history also carries this release name, retain
  # the existing dual-provenance requirement instead of allowing a GitHub tag
  # to stand in for canonical GitLab provenance.
  if git -C "$root" tag --merged "$canonical" --list "$tag" | grep -Fxq "$tag"; then
    git -C "$root" -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen -c gpg.ssh.allowedSignersFile="$gitlab_allowed_signers" verify-tag "$tag" >/dev/null 2>&1 || {
      echo "canonical GitLab tag does not verify: $tag" >&2
      exit 1
    }
  fi
  if ! git -C "$projection" -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen -c gpg.ssh.allowedSignersFile="$github_allowed_signers" verify-tag "github/$tag" >/dev/null 2>&1; then
    # Legacy GitHub provenance predates the provider-specific signer. It can
    # remain verifiable only for an explicit legacy inventory; every new
    # GitHub tag must pass the current provider trust anchor above.
    if test -f "$github_legacy_tags" && grep -Fxq "$tag" "$github_legacy_tags" && \
      git -C "$projection" -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen -c gpg.ssh.allowedSignersFile="$github_legacy_allowed_signers" verify-tag "github/$tag" >/dev/null 2>&1; then
      :
    else
      echo "GitHub provenance tag does not verify under its permitted trust anchors: $tag" >&2
      exit 1
    fi
  fi
done

# Rewritten identity histories cannot use ordinary ancestry. The lease protects
# against concurrent GitHub ref changes; provider-native tags remain untouched.
git_transport -C "$projection" -c user.name="$github_name" -c user.email="$github_email" -c user.useConfigOnly=true \
  push --force-with-lease="refs/heads/$branch:$remote_tip" github "refs/heads/$branch:refs/heads/$branch"
printf 'GitHub provider projection synchronized: %s\n' "$projected"
