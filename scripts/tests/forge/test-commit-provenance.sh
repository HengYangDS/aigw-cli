#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
checker="$root/scripts/checks/forge/check-commit-provenance.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-commit-provenance.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/home"
export HOME="$tmp/home"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=core.hooksPath
export GIT_CONFIG_VALUE_0=/dev/null

repo="$tmp/repository"
gitlab_key="$tmp/gitlab-signing"
github_key="$tmp/github-signing"
rogue_key="$tmp/rogue-signing"
gitlab_email='gitlab@example.invalid'
github_email='github@example.invalid'
mkdir -p "$tmp/allowed"
ssh-keygen -q -t ed25519 -N '' -f "$gitlab_key"
ssh-keygen -q -t ed25519 -N '' -f "$github_key"
ssh-keygen -q -t ed25519 -N '' -f "$rogue_key"
gitlab_public=$(awk '{print $1" "$2}' "$gitlab_key.pub")
github_public=$(awk '{print $1" "$2}' "$github_key.pub")
printf '%s namespaces="git" %s\n' "$gitlab_email" "$gitlab_public" > "$tmp/allowed/gitlab"
printf '%s namespaces="git" %s\n' "$github_email" "$github_public" > "$tmp/allowed/github"

git init -q -b main "$repo"
git -C "$repo" config user.name 'Provenance Fixture'
git -C "$repo" config user.email 'legacy@example.invalid'
git -C "$repo" commit --allow-empty --no-gpg-sign -qm 'historical commit'
historical=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" commit --allow-empty --no-gpg-sign -qm 'legacy floor'
floor=$(git -C "$repo" rev-parse HEAD)

git -C "$repo" checkout -q --detach "$historical"
AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
  sh "$checker" "$repo" gitlab "$floor" >/dev/null
git -C "$repo" checkout -q main

git -C "$repo" config user.email "$gitlab_email"
git -C "$repo" -c gpg.format=ssh -c user.signingkey="$gitlab_key" \
  commit --allow-empty -S -qm 'verified GitLab commit'
gitlab_head=$(git -C "$repo" rev-parse HEAD)
AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
  sh "$checker" "$repo" gitlab "$floor" >/dev/null

git -C "$repo" config user.email 'wrong@example.invalid'
git -C "$repo" -c gpg.format=ssh -c user.signingkey="$gitlab_key" \
  commit --allow-empty -S -qm 'wrong GitLab identity'
if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
  sh "$checker" "$repo" gitlab "$gitlab_head" >"$tmp/wrong-identity.out" 2>&1; then
  echo 'commit provenance accepted the wrong GitLab identity' >&2
  exit 1
fi
grep -F "must use $gitlab_email" "$tmp/wrong-identity.out" >/dev/null

git -C "$repo" reset -q --hard "$gitlab_head"
git -C "$repo" config user.email "$gitlab_email"
git -C "$repo" commit --allow-empty --no-gpg-sign -qm 'unsigned GitLab commit'
if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
  sh "$checker" "$repo" gitlab "$gitlab_head" >"$tmp/unsigned.out" 2>&1; then
  echo 'commit provenance accepted an unsigned GitLab commit' >&2
  exit 1
fi
grep -F 'does not have a trusted gitlab signature' "$tmp/unsigned.out" >/dev/null

git -C "$repo" reset -q --hard "$floor"
git -C "$repo" config user.email "$github_email"
git -C "$repo" -c gpg.format=ssh -c user.signingkey="$github_key" \
  commit --allow-empty -S -qm 'verified GitHub commit'
github_head=$(git -C "$repo" rev-parse HEAD)
AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
  sh "$checker" "$repo" github "$floor" >/dev/null

git -C "$repo" -c gpg.format=ssh -c user.signingkey="$rogue_key" \
  commit --allow-empty -S -qm 'untrusted GitHub signer'
if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
  sh "$checker" "$repo" github "$github_head" >"$tmp/untrusted.out" 2>&1; then
  echo 'commit provenance accepted an untrusted GitHub signer' >&2
  exit 1
fi
grep -F 'does not have a trusted github signature' "$tmp/untrusted.out" >/dev/null

touch "$repo/.mailmap"
if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
  sh "$checker" "$repo" github "$floor" >"$tmp/mailmap.out" 2>&1; then
  echo 'commit provenance accepted a mailmap identity overlay' >&2
  exit 1
fi
grep -F '.mailmap is forbidden' "$tmp/mailmap.out" >/dev/null
rm "$repo/.mailmap"

if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  sh "$checker" "$repo" gitlab "$floor" >"$tmp/missing-identity.out" 2>&1; then
  echo 'commit provenance accepted an implicit publication actor' >&2
  exit 1
fi
grep -F 'author email is required through AIGW_GITLAB_AUTHOR_EMAIL' "$tmp/missing-identity.out" >/dev/null

if sh "$checker" "$repo" unknown "$floor" >"$tmp/provider.out" 2>&1; then
  echo 'commit provenance accepted an unknown provider' >&2
  exit 1
fi
grep -F 'provider must be gitlab or github' "$tmp/provider.out" >/dev/null

echo 'commit provenance tests: OK'
