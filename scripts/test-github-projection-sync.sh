#!/bin/sh
# Regression contract: GitHub history projection must run in an independent
# repository and must never rewrite the canonical checkout's refs.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
sync_script="$root/scripts/git-github-mirror-sync.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-github-projection-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

home="$tmp/home"
signing_key="$HOME/.ssh/id_ed25519_signing_yheng_20260711.pub"
remote="$tmp/github-remote.git"
repository="$tmp/repository"
worktree="$tmp/canonical-worktree"
global_config="$tmp/global.gitconfig"
mkdir -p "$home"
test -f "$signing_key" || { echo "GitHub provenance fixture signing key is unavailable" >&2; exit 1; }
: > "$global_config"

export HOME="$home"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL="$global_config"

git init -q --bare "$remote"
git init -q -b main "$repository"
git -C "$repository" config user.name 'Yang HENG'
git -C "$repository" config user.email 'heng.yang.ds@hotmail.com'
git -C "$repository" config user.useConfigOnly true
printf 'initial\n' > "$repository/README.md"
git -C "$repository" add README.md
git -C "$repository" commit -qm 'initial canonical commit'
git -C "$repository" branch sync-main
git -C "$repository" worktree add -q "$worktree" sync-main

# The production synchronizer only accepts GitHub-shaped remote URLs. Rewrite
# that URL locally to an isolated test bare repository without weakening the
# production URL admission rule.
git config --file "$global_config" url."file://$remote".insteadOf git@github.com:test/aigw-cli.git
git -C "$worktree" remote add github-mirror git@github.com:test/aigw-cli.git
# Seed a distinct remote root so the synchronizer can prove all later updates
# are fast-forward only.
git -C "$worktree" remote add github-seed "file://$remote"
git -C "$worktree" push -q github-seed sync-main:sync-main
# Add a provider-native signed provenance tag to the remote. The production
# synchronizer must verify it but must never rewrite or push it.
git -C "$worktree" -c user.name=HengYang -c user.email=hengyang.2003@tsinghua.org.cn \
  -c gpg.format=ssh -c user.signingkey="$signing_key" \
  tag -s -a v0.0.1 -m 'GitHub provenance fixture' sync-main
git -C "$worktree" push -q github-seed refs/tags/v0.0.1:refs/tags/v0.0.1
git -C "$worktree" tag -d v0.0.1 >/dev/null
printf 'hengyang.2003@tsinghua.org.cn namespaces="git" %s\n' "$(awk '{print $1" "$2}' "$signing_key")" > "$tmp/github-allowed-signers"
# Keep the configured fetch URL GitHub-shaped for admission; only transport is rewritten by the isolated test config.

git_dir=$(git -C "$worktree" rev-parse --path-format=absolute --git-dir)
common_dir=$(git -C "$worktree" rev-parse --path-format=absolute --git-common-dir)
case "$git_dir" in
  "$common_dir")
    echo 'test setup did not create a linked worktree' >&2
    exit 1
    ;;
esac

snapshot_refs() {
  git -C "$worktree" for-each-ref --format='%(refname) %(objectname)' | LC_ALL=C sort
}

before_refs=$(snapshot_refs)
before_head=$(git -C "$worktree" rev-parse HEAD)
( cd "$repository" && AIGW_GITHUB_MIRROR_BRANCH=sync-main AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/github-allowed-signers" sh "$sync_script" ) >/dev/null

after_refs=$(snapshot_refs)
after_head=$(git -C "$worktree" rev-parse HEAD)
[ "$before_refs" = "$after_refs" ] || {
  echo 'GitHub projection changed canonical refs during initial sync' >&2
  exit 1
}
[ "$before_head" = "$after_head" ] || {
  echo 'GitHub projection changed canonical HEAD during initial sync' >&2
  exit 1
}
first_remote=$(git -C "$remote" rev-parse refs/heads/sync-main)

printf 'second\n' >> "$worktree/README.md"
git -C "$worktree" add README.md
git -C "$worktree" commit -qm 'second canonical commit'
# Move the canonical checkout to the new source revision before invoking the
# synchronizer; the linked worktree retains the same ref and stays clean.
git -C "$repository" reset --hard -q sync-main
before_refs=$(snapshot_refs)
before_head=$(git -C "$worktree" rev-parse HEAD)
( cd "$repository" && AIGW_GITHUB_MIRROR_BRANCH=sync-main AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/github-allowed-signers" sh "$sync_script" ) >/dev/null

after_refs=$(snapshot_refs)
after_head=$(git -C "$worktree" rev-parse HEAD)
[ "$before_refs" = "$after_refs" ] || {
  echo 'GitHub projection changed canonical refs during incremental sync' >&2
  exit 1
}
[ "$before_head" = "$after_head" ] || {
  echo 'GitHub projection changed canonical HEAD during incremental sync' >&2
  exit 1
}
second_remote=$(git -C "$remote" rev-parse refs/heads/sync-main)
git -C "$remote" merge-base --is-ancestor "$first_remote" "$second_remote" || {
  echo 'GitHub projection branch update was not fast-forward' >&2
  exit 1
}

if git -C "$worktree" log --all --format='%ae%n%ce' | grep -Fv -x 'heng.yang.ds@hotmail.com' | grep -q .; then
  echo 'canonical GitLab history identity was changed by GitHub projection' >&2
  exit 1
fi
if git -C "$remote" log sync-main --format='%ae%n%ce' | grep -Fv -x 'hengyang.2003@tsinghua.org.cn' | grep -q .; then
  echo 'GitHub projection did not rewrite complete commit identity history' >&2
  exit 1
fi

echo 'GitHub projection isolation and fast-forward contract: OK'
