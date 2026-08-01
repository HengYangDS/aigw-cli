#!/bin/sh
set -eu

usage() {
  echo 'usage: check-commit-provenance.sh <repository> <gitlab|github>' >&2
  exit 2
}

[ "$#" -eq 2 ] || usage
repository=$1
provider=$2

case "$provider" in
  gitlab)
    required_email=${AIGW_GITLAB_AUTHOR_EMAIL:-}
    allowed_signers=${AIGW_GITLAB_ALLOWED_SIGNERS:-}
    ;;
  github)
    required_email=${AIGW_GITHUB_AUTHOR_EMAIL:-}
    allowed_signers=${AIGW_GITHUB_ALLOWED_SIGNERS:-}
    ;;
  *)
    echo 'provider must be gitlab or github' >&2
    exit 2
    ;;
esac

provider_upper=$(printf '%s' "$provider" | tr '[:lower:]' '[:upper:]')
[ -n "$required_email" ] || {
  echo "$provider author email is required through AIGW_${provider_upper}_AUTHOR_EMAIL" >&2
  exit 2
}
case "$required_email" in
  *@*.*) ;;
  *) echo "$provider author email is malformed" >&2; exit 2 ;;
esac
[ -n "$allowed_signers" ] || {
  echo "$provider trust input is required through AIGW_${provider_upper}_ALLOWED_SIGNERS" >&2
  exit 2
}

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

head=$(git -C "$repository" rev-parse --verify 'HEAD^{commit}')
commits=$(git -C "$repository" rev-list --reverse --topo-order "$head")
[ -n "$commits" ] || {
  echo "$provider commit provenance: HEAD has no reachable commits" >&2
  exit 1
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
echo "$provider commit provenance: $count verified commit(s)"
