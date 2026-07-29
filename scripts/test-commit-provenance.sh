#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-commit-provenance.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-commit-provenance.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/home"
export HOME="$tmp/home"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null

repo="$tmp/repository"
gitlab_key="$tmp/gitlab-signing"
github_key="$tmp/github-signing"
rogue_key="$tmp/rogue-signing"
mkdir -p "$tmp/allowed"
ssh-keygen -q -t ed25519 -N '' -f "$gitlab_key"
ssh-keygen -q -t ed25519 -N '' -f "$github_key"
ssh-keygen -q -t ed25519 -N '' -f "$rogue_key"
gitlab_public=$(awk '{print $1" "$2}' "$gitlab_key.pub")
github_public=$(awk '{print $1" "$2}' "$github_key.pub")
printf 'heng.yang.ds@hotmail.com namespaces="git" %s\n' "$gitlab_public" > "$tmp/allowed/gitlab"
printf 'hengyang.2003@tsinghua.org.cn namespaces="git" %s\n' "$github_public" > "$tmp/allowed/github"

git init -q -b main "$repo"
git -C "$repo" config user.name 'HengYang'
git -C "$repo" config user.email 'legacy@example.invalid'
git -C "$repo" commit --allow-empty --no-gpg-sign -qm 'historical commit'
historical=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" commit --allow-empty --no-gpg-sign -qm 'legacy floor'
floor=$(git -C "$repo" rev-parse HEAD)

git -C "$repo" checkout -q --detach "$historical"
AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  sh "$checker" "$repo" gitlab "$floor" >/dev/null
git -C "$repo" checkout -q main

git -C "$repo" config user.email 'heng.yang.ds@hotmail.com'
git -C "$repo" -c gpg.format=ssh -c user.signingkey="$gitlab_key" \
  commit --allow-empty -S -qm 'verified GitLab commit'
gitlab_head=$(git -C "$repo" rev-parse HEAD)
AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  sh "$checker" "$repo" gitlab "$floor" >/dev/null

git -C "$repo" config user.email 'wrong@example.invalid'
git -C "$repo" -c gpg.format=ssh -c user.signingkey="$gitlab_key" \
  commit --allow-empty -S -qm 'wrong GitLab identity'
if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  sh "$checker" "$repo" gitlab "$gitlab_head" >"$tmp/wrong-identity.out" 2>&1; then
  echo 'commit provenance accepted the wrong GitLab identity' >&2
  exit 1
fi
grep -F 'must use heng.yang.ds@hotmail.com' "$tmp/wrong-identity.out" >/dev/null

git -C "$repo" reset -q --hard "$gitlab_head"
git -C "$repo" config user.email 'heng.yang.ds@hotmail.com'
git -C "$repo" commit --allow-empty --no-gpg-sign -qm 'unsigned GitLab commit'
if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  sh "$checker" "$repo" gitlab "$gitlab_head" >"$tmp/unsigned.out" 2>&1; then
  echo 'commit provenance accepted an unsigned GitLab commit' >&2
  exit 1
fi
grep -F 'does not have a trusted gitlab signature' "$tmp/unsigned.out" >/dev/null

git -C "$repo" reset -q --hard "$floor"
git -C "$repo" config user.email 'hengyang.2003@tsinghua.org.cn'
git -C "$repo" -c gpg.format=ssh -c user.signingkey="$github_key" \
  commit --allow-empty -S -qm 'verified GitHub commit'
github_head=$(git -C "$repo" rev-parse HEAD)
AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  sh "$checker" "$repo" github "$floor" >/dev/null

git -C "$repo" -c gpg.format=ssh -c user.signingkey="$rogue_key" \
  commit --allow-empty -S -qm 'untrusted GitHub signer'
if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  sh "$checker" "$repo" github "$github_head" >"$tmp/untrusted.out" 2>&1; then
  echo 'commit provenance accepted an untrusted GitHub signer' >&2
  exit 1
fi
grep -F 'does not have a trusted github signature' "$tmp/untrusted.out" >/dev/null

touch "$repo/.mailmap"
if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
  sh "$checker" "$repo" github "$floor" >"$tmp/mailmap.out" 2>&1; then
  echo 'commit provenance accepted a mailmap identity overlay' >&2
  exit 1
fi
grep -F '.mailmap is forbidden' "$tmp/mailmap.out" >/dev/null
rm "$repo/.mailmap"

if sh "$checker" "$repo" unknown "$floor" >"$tmp/provider.out" 2>&1; then
  echo 'commit provenance accepted an unknown provider' >&2
  exit 1
fi
grep -F 'provider must be gitlab or github' "$tmp/provider.out" >/dev/null

echo 'commit provenance tests: OK'
