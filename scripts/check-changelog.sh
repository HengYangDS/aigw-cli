#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

fail() {
  echo "CHANGELOG.md: $*" >&2
  exit 1
}

changelog=${AIGW_CHANGELOG_FILE:-CHANGELOG.md}
case "$changelog" in
  /*) ;;
  *) changelog="$root/$changelog" ;;
esac
test -f "$changelog" || fail "missing file"

first_heading=$(awk '/^## / { print; exit }' "$changelog")
test "$first_heading" = "## [Unreleased]" || \
  fail "the first release section must be ## [Unreleased]"

latest_tag=$(git describe --tags --abbrev=0 --match 'v[0-9]*' HEAD 2>/dev/null || true)
test -n "$latest_tag" || fail "cannot find a reachable v<semver> Git tag"
latest_version=${latest_tag#v}
latest_date=$(git for-each-ref "refs/tags/$latest_tag" --format='%(creatordate:short)' | head -n 1)
test -n "$latest_date" || fail "cannot read date for $latest_tag"

first_release=$(awk '
  /^## \[/ {
    if ($0 != "## [Unreleased]") {
      print
      exit
    }
  }
' "$changelog")
expected_release="## [$latest_version] - $latest_date"
test "$first_release" = "$expected_release" || fail \
  "first published section must be $expected_release (the latest reachable tag)"

# Every release entry is anchored to a reachable release tag and uses that
# tag's creation date.  The complete order must equal Git's descending version
# order, so a copied heading cannot silently become a pseudo-release.
actual_versions=$(awk '
  /^## \[/ {
    if ($0 == "## [Unreleased]") {
      next
    }
    header = $0
    sub(/^## \[/, "", header)
    split(header, parts, "] - ")
    if (length(parts) != 2 || parts[1] == "" || parts[2] == "") {
      print "malformed release heading: " $0 > "/dev/stderr"
      exit 1
    }
    print parts[1]
  }
' "$changelog")
expected_versions=$(git tag --merged HEAD --list 'v[0-9]*' --sort=-version:refname \
  | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$' \
  | sed 's/^v//' || true)
test -n "$expected_versions" || fail "cannot find reachable SemVer release tags"
test "$actual_versions" = "$expected_versions" || fail \
  "published headings must list each reachable release tag once in descending version order"

awk '
  /^## \[/ {
    if ($0 == "## [Unreleased]") {
      next
    }
    header = $0
    sub(/^## \[/, "", header)
    split(header, parts, "] - ")
    print parts[1] "|" parts[2]
  }
' "$changelog" | while IFS='|' read -r version date; do
  tag="v$version"
  tag_date=$(git for-each-ref "refs/tags/$tag" --format='%(creatordate:short)' | head -n 1)
  test "$date" = "$tag_date" || fail \
    "release $version is dated $date but $tag was created on $tag_date"
done
