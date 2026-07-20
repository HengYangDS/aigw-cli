#!/bin/sh
set -eu

repo=${1:-.}
tag=${2:?usage: check-release-tag-signature.sh [repo] <tag> [gitlab|github]}
provider=${3:-gitlab}
repo=$(CDPATH= cd -- "$repo" && pwd)

case "$provider" in
  gitlab|github) ;;
  *) echo "release tag provider must be gitlab or github: $provider" >&2; exit 2 ;;
esac
allowed_signers="$repo/packaging/release/${provider}-allowed-signers"

case "$tag" in
  v[0-9]*.*.*|github/v[0-9]*.*.*) ;;
  *) echo "release tag is malformed: $tag" >&2; exit 2 ;;
esac

case "$tag" in
  github/*)
    test "$provider" = github || {
      echo "qualified GitHub tag requires github provider: $tag" >&2
      exit 2
    }
    ;;
esac

git -C "$repo" rev-parse -q --verify "refs/tags/$tag" >/dev/null || {
  echo "release tag does not exist: $tag" >&2
  exit 1
}

test -f "$allowed_signers" || {
  echo "release tag trust anchor is missing: $allowed_signers" >&2
  exit 1
}
command -v ssh-keygen >/dev/null 2>&1 || {
  echo "SSH release-tag verification requires ssh-keygen" >&2
  exit 1
}

if [ "$(git -C "$repo" cat-file -t "$tag")" != tag ]; then
  echo "release tag must be annotated: $tag" >&2
  exit 1
fi

object=$(git -C "$repo" cat-file -p "$tag")
case "$object" in
  *"-----BEGIN SSH SIGNATURE-----"*"-----END SSH SIGNATURE-----"*) ;;
  *) echo "release tag is not SSH signed: $tag" >&2; exit 1 ;;
esac

git -C "$repo" -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c gpg.ssh.allowedSignersFile="$allowed_signers" verify-tag "$tag" >/dev/null
echo "release tag SSH signature: OK ($provider $tag)"
