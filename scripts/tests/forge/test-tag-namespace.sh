#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
checker="$root/scripts/checks/forge/check-tag-namespace.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-tag-namespace.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=core.hooksPath
export GIT_CONFIG_VALUE_0=/dev/null
fixture="$tmp/repository"

# This suite exercises three synthetic topologies (local canonical, native
# GitLab, native GitHub).  CI identity belongs to the real checkout, never to
# a fixture, so explicitly clear it for every checker invocation unless a test
# deliberately supplies a forge mode.
run_fixture_checker() {
  AIGW_TAG_NAMESPACE_FORGE="${1:-}" GITLAB_CI= GITHUB_ACTIONS= \
    AIGW_GITLAB_ALLOWED_SIGNERS="$fixture/.config/release/gitlab-allowed-signers" \
    AIGW_GITHUB_ALLOWED_SIGNERS="$fixture/.config/release/github-allowed-signers" \
    AIGW_GITHUB_LEGACY_ALLOWED_SIGNERS="$fixture/.config/release/github-legacy-allowed-signers" \
    AIGW_GITHUB_LEGACY_TAGS="$fixture/.config/release/github-legacy-tags.txt" \
    sh "$fixture/scripts/checks/forge/check-tag-namespace.sh"
}

mkdir -p "$fixture/scripts/checks/forge" "$fixture/.config/release"
cp "$checker" "$fixture/scripts/checks/forge/check-tag-namespace.sh"

gitlab_key="$tmp/gitlab-signing"
github_key="$tmp/github-signing"
ssh-keygen -q -t ed25519 -N '' -f "$gitlab_key"
ssh-keygen -q -t ed25519 -N '' -f "$github_key"
gitlab_public=$(awk '{print $1" "$2}' "$gitlab_key.pub")
github_public=$(awk '{print $1" "$2}' "$github_key.pub")
printf 'gitlab@example.invalid namespaces="git" %s\n' "$gitlab_public" \
  > "$fixture/.config/release/gitlab-allowed-signers"
printf 'github@example.invalid namespaces="git" %s\n' "$github_public" \
  > "$fixture/.config/release/github-allowed-signers"
printf 'github@example.invalid namespaces="git" %s\n' "$github_public" \
  > "$fixture/.config/release/github-legacy-allowed-signers"
printf 'v0.0.9\n' > "$fixture/.config/release/github-legacy-tags.txt"

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

# The fixture models a canonical local checkout containing both provider
# namespaces.  A hosted CI environment must not leak its forge identity into
# that topology: the checker is intentionally exercising its local detection.
if ! run_fixture_checker > "$tmp/valid.out" 2>&1; then
  cat "$tmp/valid.out" >&2
  echo 'tag namespace checker rejected canonical GitLab and qualified GitHub tags' >&2
  exit 1
fi

git -C "$fixture" tag -d github/v0.1.0 >/dev/null
if ! run_fixture_checker gitlab > "$tmp/gitlab.out" 2>&1; then
  cat "$tmp/gitlab.out" >&2
  echo 'tag namespace checker rejected native GitLab provenance' >&2
  exit 1
fi
git -C "$fixture" tag -d v0.1.0 >/dev/null
git -C "$fixture" -c user.name='GitHub fixture identity' \
  -c user.email='github@example.invalid' -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c user.signingkey="$github_key" tag -s -a v0.1.0 -m 'native GitHub release'
git -C "$fixture" remote add origin git@github.com:example/aigw-cli.git
if ! run_fixture_checker github > "$tmp/github.out" 2>&1; then
  cat "$tmp/github.out" >&2
  echo 'tag namespace checker rejected native GitHub provenance' >&2
  exit 1
fi
if ! run_fixture_checker > "$tmp/github-auto.out" 2>&1; then
  cat "$tmp/github-auto.out" >&2
  echo 'tag namespace checker did not detect a standalone GitHub checkout' >&2
  exit 1
fi
git -C "$fixture" -c user.name='GitHub fixture identity' \
  -c user.email='github@example.invalid' -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c user.signingkey="$github_key" tag -s -a github/v0.1.0 HEAD -m 'qualified GitHub release'
if run_fixture_checker github > "$tmp/github-qualified.out" 2>&1; then
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
if run_fixture_checker > "$tmp/provider.out" 2>&1; then
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
if run_fixture_checker > "$tmp/unscoped.out" 2>&1; then
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
