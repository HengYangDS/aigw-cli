#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-tag-namespace.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-tag-namespace.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
fixture="$tmp/repository"
mkdir -p "$fixture/scripts" "$fixture/packaging/release"
cp "$checker" "$fixture/scripts/check-tag-namespace.sh"

gitlab_key="$tmp/gitlab-signing"
github_key="$tmp/github-signing"
ssh-keygen -q -t ed25519 -N '' -f "$gitlab_key"
ssh-keygen -q -t ed25519 -N '' -f "$github_key"
gitlab_public=$(awk '{print $1" "$2}' "$gitlab_key.pub")
github_public=$(awk '{print $1" "$2}' "$github_key.pub")
printf 'gitlab@example.invalid namespaces="git" %s\n' "$gitlab_public" \
  > "$fixture/packaging/release/gitlab-allowed-signers"
printf 'github@example.invalid namespaces="git" %s\n' "$github_public" \
  > "$fixture/packaging/release/github-allowed-signers"
printf 'github@example.invalid namespaces="git" %s\n' "$github_public" \
  > "$fixture/packaging/release/github-legacy-allowed-signers"
printf 'v0.0.9\n' > "$fixture/packaging/release/github-legacy-tags.txt"

git init -q "$fixture"
git -C "$fixture" config user.name 'AIGW tag namespace fixture'
git -C "$fixture" config user.email 'gitlab@example.invalid'
git -C "$fixture" config commit.gpgsign false
git -C "$fixture" config tag.gpgsign false
printf 'fixture\n' > "$fixture/fixture"
git -C "$fixture" add fixture
git -C "$fixture" commit -qm fixture
git -C "$fixture" -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c user.signingkey="$gitlab_key" tag -s -a v0.1.0 -m 'canonical GitLab release'
git -C "$fixture" -c user.name='GitHub fixture identity' \
  -c user.email='github@example.invalid' -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c user.signingkey="$github_key" tag -s -a github/v0.1.0 HEAD -m 'qualified GitHub release'

if ! sh "$fixture/scripts/check-tag-namespace.sh" > "$tmp/valid.out" 2>&1; then
  cat "$tmp/valid.out" >&2
  echo 'tag namespace checker rejected canonical GitLab and qualified GitHub tags' >&2
  exit 1
fi

git -C "$fixture" tag -d github/v0.1.0 >/dev/null
if ! AIGW_TAG_NAMESPACE_FORGE=gitlab sh "$fixture/scripts/check-tag-namespace.sh" > "$tmp/gitlab.out" 2>&1; then
  cat "$tmp/gitlab.out" >&2
  echo 'tag namespace checker rejected native GitLab provenance' >&2
  exit 1
fi
git -C "$fixture" tag -d v0.1.0 >/dev/null
git -C "$fixture" -c user.name='GitHub fixture identity' \
  -c user.email='github@example.invalid' -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c user.signingkey="$github_key" tag -s -a v0.1.0 -m 'native GitHub release'
git -C "$fixture" remote add origin git@github.com:example/aigw-cli.git
if ! AIGW_TAG_NAMESPACE_FORGE=github sh "$fixture/scripts/check-tag-namespace.sh" > "$tmp/github.out" 2>&1; then
  cat "$tmp/github.out" >&2
  echo 'tag namespace checker rejected native GitHub provenance' >&2
  exit 1
fi
if ! sh "$fixture/scripts/check-tag-namespace.sh" > "$tmp/github-auto.out" 2>&1; then
  cat "$tmp/github-auto.out" >&2
  echo 'tag namespace checker did not detect a standalone GitHub checkout' >&2
  exit 1
fi
git -C "$fixture" -c user.name='GitHub fixture identity' \
  -c user.email='github@example.invalid' -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c user.signingkey="$github_key" tag -s -a github/v0.1.0 HEAD -m 'qualified GitHub release'
if AIGW_TAG_NAMESPACE_FORGE=github sh "$fixture/scripts/check-tag-namespace.sh" > "$tmp/github-qualified.out" 2>&1; then
  cat "$tmp/github-qualified.out" >&2
  echo 'tag namespace checker accepted qualified provenance in a native GitHub checkout' >&2
  exit 1
fi
grep -F 'qualified GitHub provenance is only valid in a local canonical checkout: github/v0.1.0' \
  "$tmp/github-qualified.out" >/dev/null || {
  cat "$tmp/github-qualified.out" >&2
  echo 'tag namespace checker did not identify qualified provenance in a native GitHub checkout' >&2
  exit 1
}
git -C "$fixture" tag -d github/v0.1.0 >/dev/null
git -C "$fixture" tag -d v0.1.0 >/dev/null
git -C "$fixture" remote remove origin
git -C "$fixture" -c user.name='AIGW tag namespace fixture' \
  -c user.email='gitlab@example.invalid' -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c user.signingkey="$gitlab_key" tag -s -a v0.1.0 -m 'canonical GitLab release'

git -C "$fixture" tag provider/v0.1.0 v0.1.0
if sh "$fixture/scripts/check-tag-namespace.sh" > "$tmp/provider.out" 2>&1; then
  cat "$tmp/provider.out" >&2
  echo 'tag namespace checker accepted a legacy provider alias' >&2
  exit 1
fi
grep -F 'legacy provider alias remains: provider/v0.1.0' "$tmp/provider.out" >/dev/null || {
  cat "$tmp/provider.out" >&2
  echo 'tag namespace checker did not identify the legacy provider alias' >&2
  exit 1
}
git -C "$fixture" tag -d provider/v0.1.0 >/dev/null
git -C "$fixture" tag -d v0.1.0 >/dev/null

rogue="$tmp/rogue"
ssh-keygen -q -t ed25519 -N '' -f "$rogue"
git -C "$fixture" -c user.name='GitHub fixture identity' \
  -c user.email='github@example.invalid' -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c user.signingkey="$rogue" \
  tag -s -a v0.2.0 -m 'GitHub provenance in the wrong namespace'
if sh "$fixture/scripts/check-tag-namespace.sh" > "$tmp/unscoped.out" 2>&1; then
  cat "$tmp/unscoped.out" >&2
  echo 'tag namespace checker accepted unscoped non-GitLab provenance' >&2
  exit 1
fi
grep -F 'gitlab tag does not verify: v0.2.0' "$tmp/unscoped.out" >/dev/null || {
  cat "$tmp/unscoped.out" >&2
  echo 'tag namespace checker did not identify unscoped non-GitLab provenance' >&2
  exit 1
}

echo 'release tag namespace regression: OK'
