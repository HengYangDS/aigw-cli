#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

cd "$tmp"
git init -q
git config user.name 'AIGW signature test'
git config user.email 'aigw-signature-test@example.invalid'
mkdir -p "$tmp/packaging/release"
cp "$root/packaging/release/gitlab-allowed-signers" "$tmp/packaging/release/gitlab-allowed-signers"
printf 'fixture\n' > fixture
git add fixture
git commit -qm fixture
git tag -a v0.0.0-unsigned -m unsigned

if sh "$root/scripts/check-release-tag-signature.sh" "$tmp" v0.0.0-unsigned >"$tmp/unsigned.out" 2>&1; then
  cat "$tmp/unsigned.out" >&2
  echo "unsigned annotated release tag was accepted" >&2
  exit 1
fi
grep -F 'not SSH signed' "$tmp/unsigned.out" >/dev/null || {
  cat "$tmp/unsigned.out" >&2
  echo "unsigned tag rejection did not identify the missing SSH signature" >&2
  exit 1
}

mkdir -p "$tmp/empty-home"
signed=''
for provider in gitlab github; do
  for tag in $(git -C "$root" tag --list 'v[0-9]*'); do
    if HOME="$tmp/empty-home" sh "$root/scripts/check-release-tag-signature.sh" "$root" "$tag" "$provider" >"$tmp/signed.out" 2>&1; then
      signed=$tag
      break 2
    fi
  done
done
[ -n "$signed" ] || {
  cat "$tmp/signed.out" >&2 || true
  echo "repository contains no verifiable SSH-signed release tag fixture" >&2
  exit 1
}

echo "release tag signature gate: OK ($signed)"
