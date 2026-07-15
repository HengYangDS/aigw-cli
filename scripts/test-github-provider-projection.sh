#!/bin/sh
# Regression contract for the provider-specific GitHub history projection.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
script="$root/scripts/project-github-forge.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-github-provider-projection.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

source="$tmp/source"
remote="$tmp/github.git"
projection="$tmp/bootstrap-projection"
home="$tmp/home"
global_config="$tmp/global.gitconfig"
key="$tmp/signing"
mkdir -p "$home" "$tmp/allowed"
: > "$global_config"
ssh-keygen -q -t ed25519 -N '' -f "$key"
public=$(awk '{print $1" "$2}' "$key.pub")
printf 'heng.yang.ds@hotmail.com namespaces="git" %s\n' "$public" > "$tmp/allowed/gitlab"
printf 'hengyang.2003@tsinghua.org.cn namespaces="git" %s\n' "$public" > "$tmp/allowed/github"

export HOME="$home"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL="$global_config"
git config --file "$global_config" url."file://$remote".insteadOf git@github.com:test/aigw-cli.git

git init -q --bare "$remote"
git init -q -b main "$source"
git -C "$source" config user.name 'Yang HENG'
git -C "$source" config user.email 'heng.yang.ds@hotmail.com'
git -C "$source" config user.useConfigOnly true
printf 'first\n' > "$source/README.md"
git -C "$source" add README.md
git -C "$source" commit -qm 'first canonical source commit'
git -C "$source" -c gpg.format=ssh -c user.signingkey="$key" tag -s -a v0.1.0 -m 'GitLab release identity'
canonical_tag=$(git -C "$source" rev-parse refs/tags/v0.1.0)

# Bootstrap an existing GitHub provider history and its provider-native tag.
git clone -q --no-local "file://$source" "$projection"
git -C "$projection" tag -d v0.1.0 >/dev/null
FILTER_BRANCH_SQUELCH_WARNING=1 git -C "$projection" filter-branch -f --env-filter '
  GIT_AUTHOR_NAME="HengYang"
  GIT_AUTHOR_EMAIL="hengyang.2003@tsinghua.org.cn"
  GIT_COMMITTER_NAME="HengYang"
  GIT_COMMITTER_EMAIL="hengyang.2003@tsinghua.org.cn"
' -- main >/dev/null 2>&1
git -C "$projection" for-each-ref --format='%(refname)' refs/original/ | while IFS= read -r ref; do
  git -C "$projection" update-ref -d "$ref"
done
git -C "$projection" -c user.name=HengYang -c user.email=hengyang.2003@tsinghua.org.cn \
  -c gpg.format=ssh -c user.signingkey="$key" tag -s -a v0.1.0 -m 'GitHub release identity'
git -C "$projection" remote set-url origin "file://$remote"
git -C "$projection" -c core.hooksPath=/dev/null push -q origin main refs/tags/v0.1.0
remote_tag_before=$(git -C "$remote" rev-parse refs/tags/v0.1.0)

# Advance the canonical source. The production projection must not rewrite its
# refs or its GitLab release tag while it rewrites only the isolated clone.
printf 'second\n' >> "$source/README.md"
git -C "$source" add README.md
git -C "$source" commit -qm 'second canonical source commit'
source_head_before=$(git -C "$source" rev-parse HEAD)
source_refs_before=$(git -C "$source" for-each-ref --format='%(refname) %(objectname)' | LC_ALL=C sort)
git -C "$source" remote add github git@github.com:test/aigw-cli.git

(
  cd "$source"
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
    AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
    AIGW_GITHUB_REMOTE=github \
    sh "$script" --branch main
) >/dev/null

[ "$(git -C "$source" rev-parse HEAD)" = "$source_head_before" ] || {
  echo 'projection rewrote canonical HEAD' >&2
  exit 1
}
[ "$(git -C "$source" for-each-ref --format='%(refname) %(objectname)' | LC_ALL=C sort)" = "$source_refs_before" ] || {
  echo 'projection rewrote canonical refs' >&2
  exit 1
}
[ "$(git -C "$source" rev-parse refs/tags/v0.1.0)" = "$canonical_tag" ] || {
  echo 'projection rewrote the GitLab release tag' >&2
  exit 1
}
[ "$(git -C "$remote" rev-parse refs/tags/v0.1.0)" = "$remote_tag_before" ] || {
  echo 'projection rewrote the GitHub release tag' >&2
  exit 1
}
[ "$(git -C "$remote" rev-parse refs/heads/main^{tree})" = "$(git -C "$source" rev-parse HEAD^{tree})" ] || {
  echo 'projected GitHub main tree differs from canonical source' >&2
  exit 1
}
if git -C "$remote" log main --format='%ae%n%ce' | grep -Fv -x 'hengyang.2003@tsinghua.org.cn' | grep -q .; then
  echo 'GitHub projection retains a non-GitHub identity' >&2
  exit 1
fi

echo 'GitHub provider projection isolation contract: OK'
