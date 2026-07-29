#!/bin/sh
# Project new canonical GitLab commits into the GitHub identity domain. Only
# descendants of the existing GitHub tip are created; canonical refs and
# provider-native release tags are never altered or pushed by this command.
set -eu

usage() {
  cat >&2 <<'USAGE'
usage: project-github-forge.sh [--branch <name>] [--github-remote <name>]

Projects one canonical branch into the GitHub peer repository with the GitHub
commit identity and a trusted GitHub commit signature.
Existing GitHub release tags must verify as GitHub provenance before the branch
update. The update must be a fast-forward; no tag ref is pushed and no
history-rewrite escape exists.
USAGE
  exit 2
}

branch=main
github_remote=${AIGW_GITHUB_REMOTE:-github}
github_name=${AIGW_GITHUB_AUTHOR_NAME:-HengYang}
github_email=${AIGW_GITHUB_AUTHOR_EMAIL:-hengyang.2003@tsinghua.org.cn}
script_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
release_directory="$script_root/packaging/release"
github_allowed_signers=${AIGW_GITHUB_ALLOWED_SIGNERS:-$release_directory/github-allowed-signers}
github_legacy_allowed_signers=${AIGW_GITHUB_LEGACY_ALLOWED_SIGNERS:-$release_directory/github-legacy-allowed-signers}
github_legacy_tags=${AIGW_GITHUB_LEGACY_TAGS:-$release_directory/github-legacy-tags.txt}
gitlab_allowed_signers=${AIGW_GITLAB_ALLOWED_SIGNERS:-$release_directory/gitlab-allowed-signers}
verified_commit_floors=${AIGW_VERIFIED_COMMIT_FLOORS:-$release_directory/verified-commit-floors.txt}
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
case "$github_name" in HengYang) ;; *) echo "GitHub author name must be HengYang" >&2; exit 2 ;; esac
case "$github_email" in hengyang.2003@tsinghua.org.cn) ;; *) echo "GitHub identity must be hengyang.2003@tsinghua.org.cn" >&2; exit 2 ;; esac

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "run inside a Git worktree" >&2; exit 2; }
git diff --quiet && git diff --cached --quiet || { echo "refusing GitHub projection with a dirty canonical worktree" >&2; exit 2; }

canonical_root=$(git rev-parse --show-toplevel)
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

projection_fingerprint() {
  commit=$1
  parents=$(git -C "$projection" show -s --format='%P' "$commit")
  parent_count=$(printf '%s\n' "$parents" | awk '{print NF}')
  message=$(git -C "$projection" show -s --format='%B' "$commit")
  {
    printf 'parents=%s\n' "$parent_count"
    git -C "$projection" show -s --format='%T%n%aI%n%cI' "$commit"
    printf '%s\n' "$message"
  } | git hash-object --stdin
}

# file:// forces a fresh object database. Linked worktrees, local clone
# optimisations, alternates, and shared object databases are explicitly barred.
git clone --quiet --no-local "file://$canonical_root" "$projection"
projection_common=$(git -C "$projection" rev-parse --path-format=absolute --git-common-dir)
[ "$projection_common" != "$canonical_common" ] || { echo "projection clone shares canonical Git common directory" >&2; exit 1; }
[ ! -e "$projection_common/objects/info/alternates" ] || { echo "projection clone has object alternates" >&2; exit 1; }
git -C "$projection" remote remove origin 2>/dev/null || true

# Keep exactly one canonical source ref. Provider-native tags are fetched into
# a separate namespace below and are never copied, regenerated, or pushed.
git -C "$projection" update-ref "$projection_source_ref" "$canonical"
git -C "$projection" checkout --detach --quiet "$projection_source_ref"
git -C "$projection" for-each-ref --format='delete %(refname)' refs/heads refs/tags | git -C "$projection" update-ref --stdin
test -z "$(git -C "$projection" status --porcelain)" || {
  echo "projection clone is not clean before identity projection" >&2
  exit 1
}

# The canonical plane enforces its own identity and signature only after its
# declared floor. Historical releases remain untouched.
AIGW_GITLAB_ALLOWED_SIGNERS="$gitlab_allowed_signers" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$github_allowed_signers" \
  AIGW_VERIFIED_COMMIT_FLOORS="$verified_commit_floors" \
  sh "$script_root/scripts/check-commit-provenance.sh" "$projection" gitlab >/dev/null

git -C "$projection" remote add github "$github_url"
remote_tip=$(git_transport -C "$projection" ls-remote --heads github "refs/heads/$branch" | awk 'NR==1 {print $1}')
[ -n "$remote_tip" ] || { echo "GitHub branch is missing: $branch" >&2; exit 1; }
git_transport -C "$projection" fetch --quiet --no-tags github "refs/heads/$branch:refs/remotes/github/$branch"
remote_tree=$(git -C "$projection" rev-parse "refs/remotes/github/$branch^{tree}")

# The GitHub tip is provider-specific history, so commit ancestry differs. Its
# tree must map to one canonical commit; only later canonical commits are
# eligible for forward projection.
canonical_base=$(git -C "$projection" log "$canonical" --format='%H %T' | awk -v tree="$remote_tree" '$2 == tree {print $1; exit}')
[ -n "$canonical_base" ] || {
  echo "GitHub branch tree diverges from canonical history; resolve manually" >&2
  exit 1
}

# Existing GitHub descendants after its floor must already satisfy the GitHub
# identity policy. This follows the tree classification so divergence and
# provenance remain distinct failure classes.
git -C "$projection" checkout --detach --quiet "$remote_tip"
AIGW_GITLAB_ALLOWED_SIGNERS="$gitlab_allowed_signers" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$github_allowed_signers" \
  AIGW_VERIFIED_COMMIT_FLOORS="$verified_commit_floors" \
  sh "$script_root/scripts/check-commit-provenance.sh" "$projection" github >/dev/null
git -C "$projection" checkout --detach --quiet "$projection_source_ref"

# Every GitHub release tag whose source tree is represented by the selected
# canonical branch must verify under the GitHub trust policy before the branch
# projection is updated.  This covers both same-named GitLab/GitHub releases
# and GitHub-native provenance tags created after an identity projection. Tags remain
# outside this branch-only projection; no tag is copied, regenerated, or
# pushed. Historical tags whose trees are absent from the selected branch do
# not block a current projection.
remote_tags="$workspace/remote-tags"
git_transport -C "$projection" ls-remote --tags github 'v[0-9]*' > "$remote_tags"
canonical_trees="$workspace/canonical-trees"
git -C "$canonical_root" log "$canonical" --format=%T | LC_ALL=C sort -u > "$canonical_trees"
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
  if git -C "$canonical_root" tag --merged "$canonical" --list "$tag" | grep -Fxq "$tag"; then
    git -C "$canonical_root" -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen -c gpg.ssh.allowedSignersFile="$gitlab_allowed_signers" verify-tag "$tag" >/dev/null 2>&1 || {
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

# Replay every commit after the tree-equivalent canonical base in topological
# order. A private ref maps each canonical parent to its projected GitHub
# parent, preserving merge topology without rewriting any existing GitHub
# commit. Every generated commit has the GitHub identity, a trusted signature,
# and the canonical source tree.
new_commits=$(git -C "$projection" rev-list --reverse --topo-order "$canonical_base..$canonical")
projected=$remote_tip
projection_map="refs/aigw-projection-map"
git -C "$projection" update-ref "$projection_map/$canonical_base" "$remote_tip"
if [ -n "$new_commits" ]; then
  if [ -z "$github_signing_key" ]; then
    github_signing_key=$(git -C "$canonical_root" config --get aigw.githubSigningKey 2>/dev/null || true)
  fi
  [ -n "$github_signing_key" ] || {
    echo 'GitHub signing key is required through AIGW_GITHUB_SIGNING_KEY or aigw.githubSigningKey' >&2
    exit 2
  }
  [ -f "$github_signing_key" ] || {
    echo "GitHub signing key is not a readable file: $github_signing_key" >&2
    exit 2
  }
  if [ -z "$github_signing_program" ]; then
    github_signing_program=$(git -C "$canonical_root" config --get aigw.githubSigningProgram 2>/dev/null || true)
  fi
  github_signing_program=${github_signing_program:-ssh-keygen}
  case "$github_signing_program" in
    */*) [ -x "$github_signing_program" ] || {
      echo "GitHub signing program is not executable: $github_signing_program" >&2
      exit 2
    } ;;
    *) command -v "$github_signing_program" >/dev/null 2>&1 || {
      echo "GitHub signing program is unavailable: $github_signing_program" >&2
      exit 2
    } ;;
  esac

  message_file="$workspace/commit-message"
  for source_commit in $new_commits; do
    source_parents=$(git -C "$projection" show -s --format='%P' "$source_commit")
    set --
    for source_parent in $source_parents; do
      projected_parent=$(git -C "$projection" rev-parse --verify "$projection_map/$source_parent^{commit}" 2>/dev/null || true)
      if [ -z "$projected_parent" ]; then
        source_parent_fingerprint=$(projection_fingerprint "$source_parent")
        projected_matches=
        for candidate in $(git -C "$projection" rev-list "$remote_tip"); do
          if [ "$(projection_fingerprint "$candidate")" = "$source_parent_fingerprint" ]; then
            projected_matches="${projected_matches}${projected_matches:+
}$candidate"
          fi
        done
        match_count=$(printf '%s\n' "$projected_matches" | awk 'NF {count++} END {print count + 0}')
        [ "$match_count" -eq 1 ] || {
          echo "canonical parent has $match_count identity-neutral GitHub matches: $source_parent" >&2
          exit 1
        }
        projected_parent=$projected_matches
        git -C "$projection" update-ref "$projection_map/$source_parent" "$projected_parent"
      fi
      set -- "$@" -p "$projected_parent"
    done
    source_tree=$(git -C "$projection" show -s --format='%T' "$source_commit")
    author_date=$(git -C "$projection" show -s --format='%aI' "$source_commit")
    committer_date=$(git -C "$projection" show -s --format='%cI' "$source_commit")
    source_message=$(git -C "$projection" show -s --format='%B' "$source_commit")
    printf '%s\n' "$source_message" > "$message_file"
    projected=$(
      GIT_AUTHOR_NAME="$github_name" \
      GIT_AUTHOR_EMAIL="$github_email" \
      GIT_AUTHOR_DATE="$author_date" \
      GIT_COMMITTER_NAME="$github_name" \
      GIT_COMMITTER_EMAIL="$github_email" \
      GIT_COMMITTER_DATE="$committer_date" \
      git -C "$projection" \
        -c gpg.format=ssh \
        -c gpg.ssh.program="$github_signing_program" \
        -c user.signingkey="$github_signing_key" \
        commit-tree -S "$source_tree" "$@" < "$message_file"
    )
    git -C "$projection" \
      -c gpg.format=ssh \
      -c gpg.ssh.program=ssh-keygen \
      -c gpg.ssh.allowedSignersFile="$github_allowed_signers" \
      verify-commit "$projected" >/dev/null 2>&1 || {
      echo "generated GitHub commit does not verify: $projected" >&2
      exit 1
    }
    git -C "$projection" update-ref "$projection_map/$source_commit" "$projected"
  done
fi

[ "$(git -C "$projection" rev-parse "$projected^{tree}")" = "$canonical_tree" ] || {
  echo "projected GitHub branch tree differs from canonical branch" >&2
  exit 1
}
git -C "$projection" update-ref "refs/heads/$branch" "$projected"
git -C "$projection" checkout --detach --quiet "$projected"
AIGW_GITLAB_ALLOWED_SIGNERS="$gitlab_allowed_signers" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$github_allowed_signers" \
  AIGW_VERIFIED_COMMIT_FLOORS="$verified_commit_floors" \
  sh "$script_root/scripts/check-commit-provenance.sh" "$projection" github >/dev/null

# An ordinary push is the concurrency guard and the no-rewrite guarantee: any
# remote advance or divergent ref makes this operation fail non-fast-forward.
git_transport -C "$projection" \
  -c user.name="$github_name" \
  -c user.email="$github_email" \
  -c user.useConfigOnly=true \
  push --quiet github "refs/heads/$branch:refs/heads/$branch"
printf 'GitHub provider projection synchronized: %s\n' "$projected"
