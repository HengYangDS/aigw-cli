#!/bin/sh
set -eu

usage() {
  echo 'usage: check-commit-provenance.sh <repository> <gitlab|github> [exclusive-floor]' >&2
  exit 2
}

[ "$#" -ge 2 ] && [ "$#" -le 3 ] || usage
repository=$1
provider=$2
explicit_floor=${3:-}

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
release_directory="$root/packaging/release"
floor_file=${AIGW_VERIFIED_COMMIT_FLOORS:-$release_directory/verified-commit-floors.txt}

case "$provider" in
  gitlab)
    required_email='heng.yang.ds@hotmail.com'
    allowed_signers=${AIGW_GITLAB_ALLOWED_SIGNERS:-$release_directory/gitlab-allowed-signers}
    ;;
  github)
    required_email='hengyang.2003@tsinghua.org.cn'
    allowed_signers=${AIGW_GITHUB_ALLOWED_SIGNERS:-$release_directory/github-allowed-signers}
    ;;
  *)
    echo 'provider must be gitlab or github' >&2
    exit 2
    ;;
esac

git -C "$repository" rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
  echo "not a Git repository: $repository" >&2
  exit 2
}
[ ! -e "$repository/.mailmap" ] || {
  echo '.mailmap is forbidden because provider identities must be stored in commit objects' >&2
  exit 1
}
[ -f "$allowed_signers" ] || {
  echo "allowed signers file is missing: $allowed_signers" >&2
  exit 2
}

floor=$explicit_floor
if [ -z "$floor" ]; then
  [ -f "$floor_file" ] || {
    echo "verified commit floor file is missing: $floor_file" >&2
    exit 2
  }
  floor=$(awk -v provider="$provider" '$1 == provider {print $2}' "$floor_file")
  [ -n "$floor" ] || {
    echo "verified commit floor is missing for $provider" >&2
    exit 2
  }
fi

floor=$(git -C "$repository" rev-parse --verify "$floor^{commit}" 2>/dev/null) || {
  echo "verified commit floor is not a commit: $floor" >&2
  exit 2
}
head=$(git -C "$repository" rev-parse --verify 'HEAD^{commit}')
if ! git -C "$repository" merge-base --is-ancestor "$floor" "$head"; then
  if git -C "$repository" merge-base --is-ancestor "$head" "$floor"; then
    echo "$provider commit provenance: historical revision precedes enforcement floor"
    exit 0
  fi
  echo "verified commit floor and HEAD are on divergent histories: $floor" >&2
  exit 1
fi

commits=$(git -C "$repository" rev-list --reverse "$floor..$head")
[ -n "$commits" ] || {
  echo "$provider commit provenance: no descendants after floor"
  exit 0
}

for commit in $commits; do
  author_email=$(git -C "$repository" show -s --format='%ae' "$commit")
  committer_email=$(git -C "$repository" show -s --format='%ce' "$commit")
  if [ "$author_email" != "$required_email" ] || [ "$committer_email" != "$required_email" ]; then
    echo "$provider commit $commit must use $required_email for author and committer" >&2
    exit 1
  fi
  if ! git -C "$repository" \
    -c gpg.format=ssh \
    -c gpg.ssh.program=ssh-keygen \
    -c gpg.ssh.allowedSignersFile="$allowed_signers" \
    verify-commit "$commit" >/dev/null 2>&1; then
    echo "$provider commit $commit does not have a trusted $provider signature" >&2
    exit 1
  fi
done

count=$(printf '%s\n' "$commits" | awk 'NF {count++} END {print count + 0}')
echo "$provider commit provenance: $count verified descendant(s)"
