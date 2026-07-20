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
cp "$root/packaging/release/github-allowed-signers" "$tmp/packaging/release/github-allowed-signers"
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

git update-ref refs/tags/github/v0.0.0-unsigned refs/tags/v0.0.0-unsigned
if sh "$root/scripts/check-release-tag-signature.sh" "$tmp" github/v0.0.0-unsigned github >"$tmp/github-qualified.out" 2>&1; then
  cat "$tmp/github-qualified.out" >&2
  echo "unsigned qualified GitHub tag was accepted" >&2
  exit 1
fi
grep -F 'not SSH signed' "$tmp/github-qualified.out" >/dev/null || {
  cat "$tmp/github-qualified.out" >&2
  echo "qualified GitHub tag was rejected before signature validation" >&2
  exit 1
}

if sh "$root/scripts/check-release-tag-signature.sh" "$tmp" github/v0.0.0-unsigned gitlab >"$tmp/github-qualified-provider.out" 2>&1; then
  cat "$tmp/github-qualified-provider.out" >&2
  echo "qualified GitHub tag was accepted under the GitLab provider" >&2
  exit 1
fi
grep -F 'qualified GitHub tag requires github provider' "$tmp/github-qualified-provider.out" >/dev/null || {
  cat "$tmp/github-qualified-provider.out" >&2
  echo "qualified GitHub tag provider rejection was not explicit" >&2
  exit 1
}

echo "release tag signature gate: OK ($signed; qualified namespace rejected unsigned reference)"
