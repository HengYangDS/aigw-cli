#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
gitlab_email=heng.yang.ds@hotmail.com
github_email=hengyang.2003@tsinghua.org.cn

[ "$(git -C "$repo" config --local --get user.email)" = "$gitlab_email" ] || {
  echo "canonical AIGW checkout must use the GitLab identity" >&2
  exit 1
}
[ "$(git -C "$repo" config --local --get remote.github-mirror.pushurl)" = no_direct_push_allowed ] || {
  echo "direct GitHub mirror pushes must be disabled" >&2
  exit 1
}
[ -x "$repo/scripts/git-github-mirror-sync.sh" ] || {
  echo "GitHub mirror projection command is missing" >&2
  exit 1
}
grep -Fq "$github_email" "$repo/scripts/git-github-mirror-sync.sh" || {
  echo "GitHub mirror projection must own the GitHub email" >&2
  exit 1
}
for required in 'file://$canonical_root' 'git-common-dir' 'objects/info/alternates' '--force-with-lease' 'verify-tag' 'GitHub provenance tag'; do
  grep -Fq -- "$required" "$repo/scripts/git-github-mirror-sync.sh" || {
    echo "GitHub projection safety protocol is missing: $required" >&2
    exit 1
  }
done
if grep -Fq 'refs/tags/*:refs/tags/*' "$repo/scripts/git-github-mirror-sync.sh" || grep -Fq 'push --force ' "$repo/scripts/git-github-mirror-sync.sh"; then
  echo "GitHub projection must not force-publish provenance tags" >&2
  exit 1
fi
[ "$(git -C "$repo" config --local --get remote.github-mirror.pushurl)" = no_direct_push_allowed ] || {
  echo "GitHub mirror direct-push barrier is missing from repository config" >&2
  exit 1
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
git init -q "$tmp/github"
git -C "$tmp/github" remote add origin git@github.com:HengYangDS/example.git
[ "$(git -C "$tmp/github" config --get user.email)" = "$github_email" ] || {
  echo "global GitHub identity selection failed" >&2
  exit 1
}
git init -q "$tmp/gitlab"
git -C "$tmp/gitlab" remote add origin ssh://git@192.168.64.101:1122/dig/example.git
[ "$(git -C "$tmp/gitlab" config --get user.email)" = "$gitlab_email" ] || {
  echo "global GitLab identity selection failed" >&2
  exit 1
}

hook=$(git config --global --get core.hooksPath)
[ -x "$hook/pre-push" ] || {
  echo "global provider identity push guard is missing" >&2
  exit 1
}

echo "Git provider identity mechanism: OK"
