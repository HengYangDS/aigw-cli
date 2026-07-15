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

# A release pipeline must validate its selected tag, not an arbitrary older tag
# reachable from its checkout. Branch verification keeps the latest-reachable
# rule. CI providers expose the selected tag through different variables; the
# explicit AIGW variable is also available for deterministic local rehearsal.
selected_tag=${AIGW_CHANGELOG_RELEASE_TAG:-${CI_COMMIT_TAG:-}}
if test -z "$selected_tag" && test "${GITHUB_REF_TYPE:-}" = tag; then
  selected_tag=${GITHUB_REF_NAME:-}
fi
case "$selected_tag" in
  '') ;;
  v[0-9]*.*.*) ;;
  *) fail "selected release tag is malformed: $selected_tag" ;;
esac

# CI can fetch a shallow branch whose tag object and historical ancestor are
# absent. Refresh release refs and history once before judging chronology;
# failure remains closed when no reachable tag becomes available.
latest_tag=$selected_tag
if test -z "$latest_tag"; then
  latest_tag=$(git describe --tags --abbrev=0 --match 'v[0-9]*' HEAD 2>/dev/null || true)
fi
if { test -z "$latest_tag" || ! git rev-parse -q --verify "refs/tags/$latest_tag" >/dev/null 2>&1; } && git remote get-url origin >/dev/null 2>&1; then
  git fetch --quiet --no-tags --unshallow origin 2>/dev/null || \
    git fetch --quiet --no-tags origin 2>/dev/null || true
  git fetch --quiet --no-tags origin 'refs/tags/*:refs/tags/*' 2>/dev/null || true
  if test -z "$selected_tag"; then
    latest_tag=$(git describe --tags --abbrev=0 --match 'v[0-9]*' HEAD 2>/dev/null || true)
  fi
fi
if test -n "$selected_tag"; then
  git rev-parse -q --verify "refs/tags/$selected_tag" >/dev/null || fail "selected release tag is unavailable: $selected_tag"
  test "$(git rev-parse "$selected_tag^{}")" = "$(git rev-parse HEAD)" || \
    fail "selected release tag does not identify HEAD: $selected_tag"
  latest_tag=$selected_tag
elif test -z "$latest_tag"; then
  fail "cannot find a reachable v<semver> Git tag"
fi
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

# Every release entry is anchored to a tag and uses that tag's creation date.
# In a shallow CI checkout, older tags can be unavailable even though their
# historical sections remain mandatory; therefore exact coverage is checked
# against every local release tag, while the newest reachable tag is checked
# above.  A full clone naturally verifies the complete known tag set.
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
expected_versions=$(git tag --list 'v[0-9]*' --sort=-version:refname \
  | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$' \
  | sed 's/^v//' || true)
test -n "$expected_versions" || fail "cannot find SemVer release tags"
available_heading_versions=$(printf '%s\n' "$actual_versions" | while IFS= read -r version; do
  if git rev-parse -q --verify "refs/tags/v$version" >/dev/null; then
    printf '%s\n' "$version"
  elif test "$(git rev-parse --is-shallow-repository)" = false; then
    fail "release $version has no matching Git tag v$version"
  fi
done)
expected_heading_versions=$(printf '%s\n' "$expected_versions" | while IFS= read -r version; do
  if printf '%s\n' "$available_heading_versions" | grep -Fxq "$version"; then
    printf '%s\n' "$version"
  fi
done)
test "$available_heading_versions" = "$expected_heading_versions" || fail \
  "locally available release tags must appear once in descending version order"

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
  if ! git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
    test "$(git rev-parse --is-shallow-repository)" = true && continue
    fail "release $version has no matching Git tag $tag"
  fi
  tag_date=$(git for-each-ref "refs/tags/$tag" --format='%(creatordate:short)' | head -n 1)
  test "$date" = "$tag_date" || fail \
    "release $version is dated $date but $tag was created on $tag_date"
done
