#!/bin/sh
set -eu

repo=${1:-.}
tag=${2:?usage: check-release-tag-signature.sh [repo] <tag>}

case "$tag" in
  v[0-9]*.*.*) ;;
  *) echo "release tag is malformed: $tag" >&2; exit 2 ;;
esac

git -C "$repo" rev-parse -q --verify "refs/tags/$tag" >/dev/null || {
  echo "release tag does not exist: $tag" >&2
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

git -C "$repo" verify-tag "$tag" >/dev/null
echo "release tag SSH signature: OK ($tag)"
