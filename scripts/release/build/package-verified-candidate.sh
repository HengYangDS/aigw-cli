#!/bin/sh
# Package a complete formal matrix as a non-release, offline candidate carrier.
set -eu

usage() {
  cat >&2 <<'USAGE'
usage: package-verified-candidate.sh <candidate-version> <formal-dist-dir> <candidate-output-dir>

The formal dist directory remains exactly the 15 release artifacts. The output
is a separate candidate carrier and its outer SHA-256 digest; neither is a tag
or a published release.
USAGE
  exit 2
}

[ "$#" = 3 ] || usage
version=$1
dist=$2
out=$3
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)

case "$version" in
  *[!0-9A-Za-z._-]*|'') echo "invalid candidate version: $version" >&2; exit 2 ;;
esac
[ -d "$dist" ] || { echo "formal dist directory does not exist: $dist" >&2; exit 2; }

dist_abs=$(CDPATH= cd -- "$dist" && pwd)
out_parent=$(CDPATH= cd -- "$(dirname -- "$out")" && pwd)
out_abs="$out_parent/$(basename -- "$out")"
case "$out_abs" in "$dist_abs"|"$dist_abs"/*)
  echo "candidate output directory must be outside formal dist" >&2
  exit 2
esac

"$root/scripts/checks/release/check-release-artifacts.sh" "$dist_abs" "$version" >/dev/null
git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
  echo "candidate packaging requires a Git worktree" >&2
  exit 2
}
git -C "$root" diff --quiet && git -C "$root" diff --cached --quiet || {
  echo "candidate packaging requires a clean tracked worktree" >&2
  exit 1
}

commit=$(git -C "$root" rev-parse HEAD)
tree=$(git -C "$root" rev-parse HEAD^{tree})
timestamp=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
carrier_name="aigw_${version}_candidate_${commit}.tar.gz"
carrier_root=${carrier_name%.tar.gz}
stage=$(mktemp -d "${TMPDIR:-/tmp}/aigw-candidate.XXXXXX")
trap 'rm -rf "$stage"' EXIT HUP INT TERM
payload="$stage/$carrier_root"
mkdir -p "$payload/artifacts" "$out_abs"

cp "$dist_abs"/* "$payload/artifacts/"
for script in \
  "$root/scripts/checks/release/check-release-artifacts.sh" \
  "$root/scripts/checks/release/check-verified-candidate.sh"
do
  name=$(basename -- "$script")
  cp "$script" "$payload/$name"
  chmod 755 "$payload/$name"
done

if command -v sha256sum >/dev/null 2>&1; then
  manifest_digest=$(sha256sum "$payload/artifacts/checksums.txt" | awk '{print tolower($1)}')
else
  manifest_digest=$(shasum -a 256 "$payload/artifacts/checksums.txt" | awk '{print tolower($1)}')
fi
cat > "$payload/candidate.json" <<EOF
{
  "schema": 1,
  "kind": "aigw-verified-candidate",
  "version": "$version",
  "commit": "$commit",
  "tree": "$tree",
  "created_utc": "$timestamp",
  "artifacts_dir": "artifacts",
  "checksums_path": "artifacts/checksums.txt",
  "checksums_sha256": "$manifest_digest",
  "artifact_count": 15
}
EOF

rm -f "$out_abs/$carrier_name" "$out_abs/$carrier_name.sha256"
(cd "$stage" && tar -czf "$out_abs/$carrier_name" "$carrier_root")
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$out_abs" && sha256sum "$carrier_name" > "$carrier_name.sha256")
else
  (cd "$out_abs" && shasum -a 256 "$carrier_name" > "$carrier_name.sha256")
fi

printf 'verified candidate carrier written to %s\n' "$out_abs/$carrier_name"
