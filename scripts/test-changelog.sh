#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-changelog.sh"
fixture=$(mktemp "${TMPDIR:-/tmp}/aigw-changelog.XXXXXX")
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-changelog.XXXXXX")
trap 'rm -f "$fixture"; rm -rf "$tmp"' EXIT HUP INT TERM

# This suite exercises branch chronology fixtures.  A surrounding tag pipeline
# must not turn those fixtures into a selected-tag test accidentally.
run_branch_checker() {
  AIGW_CHANGELOG_FORGE= GITHUB_ACTIONS= GITLAB_CI= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= AIGW_CHANGELOG_RELEASE_TAG= \
    AIGW_CHANGELOG_FILE="$1" sh "$checker"
}

cp "$root/CHANGELOG.md" "$fixture"
run_branch_checker "$fixture"

# A shallow checkout can omit both the release tag and its historical commit.
# Build a complete, minimal release history first, then clone it shallowly. The
# source checkout may itself be shallow in CI, so it cannot safely serve as the
# fixture's remote history.
shallow="$tmp/shallow"
origin="$tmp/origin.git"
source="$tmp/source"
git init -q -b main "$source"
git -C "$source" config user.name 'AIGW Changelog Test'
git -C "$source" config user.email 'aigw-changelog-test@example.invalid'
printf 'release\n' > "$source/release.txt"
git -C "$source" add release.txt
git -C "$source" commit -qm 'release'
git -C "$source" tag -a v0.1.0-rc.1 -m 'release'
release_date=$(git -C "$source" show -s --format=%cs v0.1.0-rc.1^{})
test -n "$release_date" || { echo "fixture release tag has no date" >&2; exit 1; }
printf 'current\n' >> "$source/release.txt"
git -C "$source" commit -qam 'current'
git init -q --bare "$origin"
git -C "$source" push -q "$origin" 'HEAD:refs/heads/main' --tags
git -C "$origin" symbolic-ref HEAD refs/heads/main
git clone -q --depth 1 --branch main "file://$origin" "$shallow"
cat > "$shallow/CHANGELOG.md" <<EOF
# Changelog

## [Unreleased]

## [0.1.0-rc.1] - $release_date
EOF
mkdir -p "$shallow/scripts"
cp "$checker" "$shallow/scripts/check-changelog.sh"
(
  cd "$shallow"
  test "$(git rev-parse --is-shallow-repository)" = true
  test -z "$(git tag --list 'v[0-9]*')"
  # This fixture intentionally has no selected release tag. Clear the outer
  # CI tag variables so a tag pipeline cannot leak its real release identity
  # into the shallow branch-history assertion.
  AIGW_CHANGELOG_FORGE= GITHUB_ACTIONS= GITLAB_CI= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= \
    AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh
  if AIGW_CHANGELOG_RELEASE_TAG=v0.1.0-rc.1 AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh >/dev/null 2>&1; then
    echo "changelog checker accepted a selected tag that does not identify shallow HEAD" >&2
    exit 1
  fi
  if AIGW_CHANGELOG_RELEASE_TAG=v0.1.0-rc.2 AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh >/dev/null 2>&1; then
    echo "changelog checker accepted a missing selected release tag" >&2
    exit 1
  fi
)

# A selected tag can exist locally during release admission before it is pushed
# to origin. Refreshing origin's tag namespace must not discard that selected
# local provenance object before the checker validates it.
prepush="$tmp/prepush"
prepush_origin="$tmp/prepush-origin.git"
git init -q -b main "$prepush"
git -C "$prepush" config user.name 'AIGW Changelog Test'
git -C "$prepush" config user.email 'aigw-changelog-test@example.invalid'
printf 'release\n' > "$prepush/release.txt"
git -C "$prepush" add release.txt
GIT_AUTHOR_DATE='2026-07-17T00:00:00Z' GIT_COMMITTER_DATE='2026-07-17T00:00:00Z' \
  git -C "$prepush" commit -qm 'release'
GIT_COMMITTER_DATE='2026-07-17T00:00:00Z' \
  git -C "$prepush" tag -a v0.1.0-rc.1 -m 'release'
git init -q --bare "$prepush_origin"
git -C "$prepush" remote add origin "file://$prepush_origin"
git -C "$prepush" push -q origin 'HEAD:refs/heads/main' refs/tags/v0.1.0-rc.1
printf 'candidate\n' >> "$prepush/release.txt"
git -C "$prepush" add release.txt
GIT_AUTHOR_DATE='2026-07-17T00:01:00Z' GIT_COMMITTER_DATE='2026-07-17T00:01:00Z' \
  git -C "$prepush" commit -qm 'candidate'
GIT_COMMITTER_DATE='2026-07-17T00:01:00Z' \
  git -C "$prepush" tag -a v0.1.0-rc.2 -m 'local pre-push candidate'
cat > "$prepush/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

## [0.1.0-rc.2] - 2026-07-17

## [0.1.0-rc.1] - 2026-07-17
EOF
mkdir -p "$prepush/scripts"
cp "$checker" "$prepush/scripts/check-changelog.sh"
(
  cd "$prepush"
  test "$(git tag --list v0.1.0-rc.2)" = v0.1.0-rc.2
  AIGW_CHANGELOG_FORGE=gitlab GITHUB_ACTIONS= GITLAB_CI= \
    AIGW_CHANGELOG_RELEASE_TAG=v0.1.0-rc.2 sh scripts/check-changelog.sh
  test "$(git tag --list v0.1.0-rc.2)" = v0.1.0-rc.2
)

# A branch checkout can retain an older reachable tag while a newer release
# tag is present only on origin. The checker must refresh tag refs before it
# classifies shared historical headings as untagged candidates.
stale_source="$tmp/stale-source"
stale_origin="$tmp/stale-origin.git"
stale="$tmp/stale"
git init -q -b main "$stale_source"
git -C "$stale_source" config user.name 'AIGW Changelog Test'
git -C "$stale_source" config user.email 'aigw-changelog-test@example.invalid'
printf 'rc1\n' > "$stale_source/release.txt"
git -C "$stale_source" add release.txt
git -C "$stale_source" commit -qm 'rc1'
GIT_COMMITTER_DATE='2026-07-17T00:00:00Z' \
  git -C "$stale_source" tag -a v0.1.0-rc.1 -m 'rc1'
printf 'rc2\n' >> "$stale_source/release.txt"
git -C "$stale_source" commit -qam 'rc2'
GIT_COMMITTER_DATE='2026-07-17T00:01:00Z' \
  git -C "$stale_source" tag -a v0.1.0-rc.2 -m 'rc2'
git init -q --bare "$stale_origin"
git -C "$stale_source" push -q "$stale_origin" 'HEAD:refs/heads/main' --tags
git -C "$stale_origin" symbolic-ref HEAD refs/heads/main
git clone -q "file://$stale_origin" "$stale"
git -C "$stale" checkout -q v0.1.0-rc.1
git -C "$stale" tag -d v0.1.0-rc.2 >/dev/null
cat > "$stale/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

## [0.1.0-rc.3] - 2026-07-17

## [0.1.0-rc.2] - 2026-07-17

## [0.1.0-rc.1] - 2026-07-17
EOF
mkdir -p "$stale/scripts"
cp "$checker" "$stale/scripts/check-changelog.sh"
if ! (
  cd "$stale"
  test "$(git describe --tags --abbrev=0 --match 'v[0-9]*' HEAD)" = v0.1.0-rc.1
  test -z "$(git tag --list v0.1.0-rc.2)"
  AIGW_CHANGELOG_FORGE= GITHUB_ACTIONS= GITLAB_CI= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= \
    AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh
); then
  echo "changelog checker did not refresh a newer remote release tag" >&2
  exit 1
fi

# A version retired from GitLab can remain an active provenance tag on GitHub.
# The GitHub checker must use its active tag instead of treating the shared
# GitLab retirement inventory as a conflicting duplicate.
github_overlap="$tmp/github-active-retired"
git init -q -b main "$github_overlap"
git -C "$github_overlap" config user.name 'AIGW Changelog Test'
git -C "$github_overlap" config user.email 'aigw-changelog-test@example.invalid'
printf 'release\n' > "$github_overlap/release.txt"
git -C "$github_overlap" add release.txt
git -C "$github_overlap" commit -qm 'release'
GIT_COMMITTER_DATE='2026-07-17T00:00:00Z' \
  git -C "$github_overlap" tag -a v0.1.0-rc.2 -m 'active GitHub release'
printf 'current\n' >> "$github_overlap/release.txt"
git -C "$github_overlap" commit -qam 'current'
cat > "$github_overlap/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

## [0.1.0-rc.3] - 2026-07-17

## [0.1.0-rc.2] - 2026-07-17
EOF
mkdir -p "$github_overlap/scripts" "$github_overlap/packaging/release"
cp "$checker" "$github_overlap/scripts/check-changelog.sh"
printf 'v0.1.0-rc.2\n' > "$github_overlap/packaging/release/retired-gitlab-tags.txt"
if ! (
  cd "$github_overlap"
  AIGW_CHANGELOG_FORGE= GITLAB_CI= GITHUB_ACTIONS=true \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= \
    AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh
); then
  echo "changelog checker rejected an active GitHub tag in the GitLab retirement inventory" >&2
  exit 1
fi

# A GitLab-only checkout cannot fetch private GitHub provenance tags during its
# source gate. The explicit GitHub-only inventory must preserve the published
# Changelog section as known history without admitting a second untagged
# candidate. A GitHub checkout with the active tag must accept the same shared
# inventory without treating it as a duplicate.
github_only="$tmp/github-only"
git init -q -b main "$github_only"
git -C "$github_only" config user.name 'AIGW Changelog Provider Test'
git -C "$github_only" config user.email 'aigw-changelog-provider@example.invalid'
printf 'gitlab release\n' > "$github_only/release.txt"
git -C "$github_only" add release.txt
git -C "$github_only" commit -qm 'GitLab release'
git -C "$github_only" tag -a v0.1.0-rc.1 -m 'GitLab release'
printf 'current\n' >> "$github_only/release.txt"
git -C "$github_only" commit -qam 'current'
cat > "$github_only/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

## [0.1.0-rc.2] - 2026-07-17

## [0.1.0-rc.1] - 2026-07-17
EOF
mkdir -p "$github_only/scripts" "$github_only/packaging/release"
cp "$checker" "$github_only/scripts/check-changelog.sh"
printf '' > "$github_only/packaging/release/retired-gitlab-tags.txt"
printf 'v0.1.0-rc.2\n' > "$github_only/packaging/release/github-only-tags.txt"
(
  cd "$github_only"
  AIGW_CHANGELOG_FORGE=gitlab GITHUB_ACTIONS= GITLAB_CI=true \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= AIGW_CHANGELOG_RELEASE_TAG= \
    sh scripts/check-changelog.sh
)
git -C "$github_only" tag -a github/v0.1.0-rc.2 -m 'GitHub-only release'
git -C "$github_only" tag -a github/v0.1.0-rc.1 HEAD~1 -m 'GitHub historical release'
(
  cd "$github_only"
  AIGW_CHANGELOG_FORGE=github GITHUB_ACTIONS=true GITLAB_CI= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= AIGW_CHANGELOG_RELEASE_TAG= \
    sh scripts/check-changelog.sh
)
if (
  cd "$github_overlap"
  AIGW_CHANGELOG_FORGE= GITHUB_ACTIONS= GITLAB_CI=true \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= \
    AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh >/dev/null 2>&1
); then
  echo "changelog checker accepted an active GitLab tag in the retired GitLab inventory" >&2
  exit 1
fi

# Exactly one untagged release candidate may be the first published section.
# A speculative heading anywhere else must fail, while a valid next candidate
# must pass before either forge creates its provider-native tag.
python3 - "$fixture" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
import subprocess

latest_tag = subprocess.check_output(
    ["git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*", "HEAD"],
    text=True,
).strip()
latest_version = latest_tag.removeprefix("v")
marker = re.search(rf"^## \[{re.escape(latest_version)}\] - \d{{4}}-\d{{2}}-\d{{2}}$", text, re.MULTILINE)
if marker is None:
    raise SystemExit("fixture lacks latest release heading")
path.write_text(text[:marker.end()] + "\n\n## [9.9.9] - 2026-07-14" + text[marker.end():], encoding="utf-8")
PY
if run_branch_checker "$fixture" >/dev/null 2>&1; then
  echo "changelog checker accepted a non-leading untagged published version" >&2
  exit 1
fi

cp "$root/CHANGELOG.md" "$fixture"
python3 - "$fixture" "$root/packaging/release/retired-gitlab-tags.txt" "$root/packaging/release/github-only-tags.txt" <<'PY'
from pathlib import Path
import re
import subprocess
import sys

path = Path(sys.argv[1])
retired = Path(sys.argv[2])
github_only = Path(sys.argv[3])
text = path.read_text(encoding="utf-8")
tag_versions = {
    tag.removeprefix("github/").removeprefix("v")
    for pattern in ("v[0-9]*", "github/v[0-9]*")
    for tag in subprocess.check_output(["git", "tag", "--list", pattern], text=True).splitlines()
}
retired_versions = {
    line.strip().removeprefix("v")
    for line in retired.read_text(encoding="utf-8").splitlines()
    if line.strip() and not line.lstrip().startswith("#")
}
github_only_versions = {
    line.strip().removeprefix("v")
    for line in github_only.read_text(encoding="utf-8").splitlines()
    if line.strip() and not line.lstrip().startswith("#")
}
published = list(re.finditer(r"^## \[([^]]+)\] - \d{4}-\d{2}-\d{2}$", text, re.MULTILINE))
unknown = [match for match in published if match.group(1) not in tag_versions | retired_versions | github_only_versions]
candidate = "## [9999.0.0-ci.1] - 2026-07-17"
if not unknown:
    unreleased = re.search(r"^## \[Unreleased\]$", text, re.MULTILINE)
    if unreleased is None:
        raise SystemExit("fixture lacks the Unreleased heading")
    text = text[:unreleased.end()] + "\n\n" + candidate + text[unreleased.end():]
elif len(unknown) == 1:
    marker = unknown[0]
    text = text[:marker.start()] + candidate + text[marker.end():]
else:
    raise SystemExit("fixture contains multiple untagged release candidates")
path.write_text(text, encoding="utf-8")
PY
# This fixture models an ordinary release-preparation branch. It must not
# inherit the outer tagged pipeline's selected release identity.
if ! run_branch_checker "$fixture" >/dev/null 2>&1; then
  echo "changelog checker rejected a leading next release candidate" >&2
  exit 1
fi

cp "$root/CHANGELOG.md" "$fixture"
python3 - "$fixture" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
pattern = re.compile(r"^(## \[(?!Unreleased\])[^]]+\]) - \d{4}-\d{2}-\d{2}$", re.MULTILINE)
changed, count = pattern.subn(r"\1 - 2000-02-30", text, count=1)
if count != 1:
    raise SystemExit("fixture lacks a published release heading")
path.write_text(changed, encoding="utf-8")
PY
if run_branch_checker "$fixture" >/dev/null 2>&1; then
  echo "changelog checker accepted an invalid release date" >&2
  exit 1
fi

# GitHub retains historical provider-native tags that GitLab has deliberately
# retired after failed candidates. The GitLab retirement inventory must act as
# a fallback, not conflict with GitHub's active historical tags.
provider="$tmp/provider"
git init -q -b main "$provider"
git -C "$provider" config user.name 'AIGW Changelog Provider Test'
git -C "$provider" config user.email 'aigw-changelog-provider@example.invalid'
git -C "$provider" commit --allow-empty -qm provider
for number in $(seq 48 58); do
  git -C "$provider" tag "v0.1.0-rc.$number"
done
mkdir -p "$provider/scripts" "$provider/packaging/release"
cp "$checker" "$provider/scripts/check-changelog.sh"
cp "$root/packaging/release/retired-gitlab-tags.txt" "$provider/packaging/release/retired-gitlab-tags.txt"
{
  printf '# Changelog\n\n## [Unreleased]\n\n'
  printf '## [0.1.0-rc.62] - 2026-07-17\n\n'
  printf '## [0.1.0-rc.61] - 2026-07-17\n\n'
  for number in $(seq 58 -1 48); do
    printf '## [0.1.0-rc.%s] - 2026-07-17\n\n' "$number"
  done
} > "$provider/CHANGELOG.md"
(
  cd "$provider"
  AIGW_CHANGELOG_FORGE=github GITHUB_ACTIONS=true GITLAB_CI= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= AIGW_CHANGELOG_RELEASE_TAG= \
    sh scripts/check-changelog.sh
)

# GitHub provider branches rewrite identities.  A provider-native tag can be
# unreachable by commit ancestry while its exact tree is present in the current
# branch.  GitHub branch CI must recognize that signed provider tag as the
# latest published source version rather than rejecting a valid Changelog.
provider_tree="$tmp/provider-tree"
provider_tree_origin="$tmp/provider-tree-origin.git"
git init -q -b main "$provider_tree"
git -C "$provider_tree" config user.name 'AIGW Changelog Provider Test'
git -C "$provider_tree" config user.email 'aigw-changelog-provider@example.invalid'
printf 'release\n' > "$provider_tree/release.txt"
git -C "$provider_tree" add release.txt
git -C "$provider_tree" commit -qm 'release'
git -C "$provider_tree" tag -a v0.1.0-rc.1 -m 'provider release'
git init -q --bare "$provider_tree_origin"
git -C "$provider_tree" remote add origin "file://$provider_tree_origin"
git -C "$provider_tree" push -q origin 'HEAD:refs/heads/main' --tags
git -C "$provider_tree" checkout -q --orphan projected
git -C "$provider_tree" rm -q --cached -r .
git -C "$provider_tree" checkout -q v0.1.0-rc.1 -- release.txt
git -C "$provider_tree" add release.txt
git -C "$provider_tree" commit -qm 'rewritten provider branch'
git -C "$provider_tree" branch -f main HEAD
git -C "$provider_tree" checkout -q main
cat > "$provider_tree/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

## [0.1.0-rc.1] - 2026-07-17
EOF
mkdir -p "$provider_tree/scripts" "$provider_tree/packaging/release"
cp "$checker" "$provider_tree/scripts/check-changelog.sh"
printf '' > "$provider_tree/packaging/release/retired-gitlab-tags.txt"
(
  cd "$provider_tree"
  AIGW_CHANGELOG_FORGE=github GITHUB_ACTIONS=true GITLAB_CI= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= AIGW_CHANGELOG_RELEASE_TAG= \
    sh scripts/check-changelog.sh
)

# A canonical checkout fetches GitHub provenance into a qualified namespace so
# it cannot overwrite an identically named GitLab tag. The GitHub chronology
# check must recognize that qualified tag, while a GitLab check must not.
provider_scoped="$tmp/provider-scoped"
git init -q -b main "$provider_scoped"
git -C "$provider_scoped" config user.name 'AIGW Changelog Provider Test'
git -C "$provider_scoped" config user.email 'aigw-changelog-provider@example.invalid'
printf 'release\n' > "$provider_scoped/release.txt"
git -C "$provider_scoped" add release.txt
git -C "$provider_scoped" commit -qm 'qualified provider release'
git -C "$provider_scoped" tag -a github/v0.1.0-rc.1 -m 'qualified GitHub provider release'
cat > "$provider_scoped/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

## [0.1.0-rc.1] - 2026-07-17
EOF
mkdir -p "$provider_scoped/scripts" "$provider_scoped/packaging/release"
cp "$checker" "$provider_scoped/scripts/check-changelog.sh"
printf '' > "$provider_scoped/packaging/release/retired-gitlab-tags.txt"
(
  cd "$provider_scoped"
  AIGW_CHANGELOG_FORGE=github GITHUB_ACTIONS=true GITLAB_CI= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= AIGW_CHANGELOG_RELEASE_TAG= \
    sh scripts/check-changelog.sh
)
if (
  cd "$provider_scoped"
  AIGW_CHANGELOG_FORGE=gitlab GITHUB_ACTIONS= GITLAB_CI=true \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= AIGW_CHANGELOG_RELEASE_TAG= \
    sh scripts/check-changelog.sh >/dev/null 2>&1
); then
  echo "changelog checker accepted a qualified GitHub tag as GitLab provenance" >&2
  exit 1
fi

# A qualified GitHub namespace must isolate chronology from higher GitLab-only
# versions, rather than allowing unscoped tags to change GitHub's latest tag.
provider_conflict="$tmp/provider-conflict"
git init -q -b main "$provider_conflict"
git -C "$provider_conflict" config user.name 'AIGW Changelog Provider Test'
git -C "$provider_conflict" config user.email 'aigw-changelog-provider@example.invalid'
printf 'gitlab\n' > "$provider_conflict/release.txt"
git -C "$provider_conflict" add release.txt
git -C "$provider_conflict" commit -qm 'GitLab release'
git -C "$provider_conflict" tag -a v0.1.0-rc.2 -m 'GitLab release'
printf 'github\n' > "$provider_conflict/release.txt"
git -C "$provider_conflict" commit -am 'GitHub release' -q
git -C "$provider_conflict" tag -a github/v0.1.0-rc.1 -m 'GitHub release'
cat > "$provider_conflict/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

## [0.1.0-rc.1] - 2026-07-17
EOF
mkdir -p "$provider_conflict/scripts" "$provider_conflict/packaging/release"
cp "$checker" "$provider_conflict/scripts/check-changelog.sh"
printf '' > "$provider_conflict/packaging/release/retired-gitlab-tags.txt"
(
  cd "$provider_conflict"
  AIGW_CHANGELOG_FORGE=github GITHUB_ACTIONS=true GITLAB_CI= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= AIGW_CHANGELOG_RELEASE_TAG= \
    sh scripts/check-changelog.sh
)

# Once a qualified GitHub namespace exists, the unscoped `git describe` result
# must not leak a GitLab-only version back into GitHub chronology. Listing both
# headings would incorrectly pass if that direct tag were re-added.
provider_latest_leak="$tmp/provider-latest-leak"
git init -q -b main "$provider_latest_leak"
git -C "$provider_latest_leak" config user.name 'AIGW Changelog Provider Test'
git -C "$provider_latest_leak" config user.email 'aigw-changelog-provider@example.invalid'
printf 'gitlab\n' > "$provider_latest_leak/release.txt"
git -C "$provider_latest_leak" add release.txt
git -C "$provider_latest_leak" commit -qm 'GitLab release'
git -C "$provider_latest_leak" tag -a v0.1.0-rc.1 -m 'GitLab release'
printf 'github\n' > "$provider_latest_leak/release.txt"
git -C "$provider_latest_leak" commit -am 'GitHub release' -q
git -C "$provider_latest_leak" tag -a github/v0.1.0-rc.2 -m 'GitHub release'
cat > "$provider_latest_leak/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

## [0.1.0-rc.2] - 2026-07-19

## [0.1.0-rc.1] - 2026-07-18
EOF
mkdir -p "$provider_latest_leak/scripts" "$provider_latest_leak/packaging/release"
cp "$checker" "$provider_latest_leak/scripts/check-changelog.sh"
printf '' > "$provider_latest_leak/packaging/release/retired-gitlab-tags.txt"
if (
  cd "$provider_latest_leak"
  AIGW_CHANGELOG_FORGE=github GITHUB_ACTIONS=true GITLAB_CI= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= AIGW_CHANGELOG_RELEASE_TAG= \
    sh scripts/check-changelog.sh 2>error.log
); then
  echo "changelog checker accepted an unscoped latest tag on the GitHub plane" >&2
  exit 1
fi
grep -Fq 'only the first release heading may be an untagged next candidate' \
  "$provider_latest_leak/error.log" || {
    cat "$provider_latest_leak/error.log" >&2
    echo "changelog checker rejected the latest-tag leak for an unexpected reason" >&2
    exit 1
  }

echo "changelog chronology contract: OK"
