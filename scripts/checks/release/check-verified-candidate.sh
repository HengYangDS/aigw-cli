#!/bin/sh
# Validate a complete candidate carrier before a human consumes its contents.
set -eu

carrier=${1:?usage: check-verified-candidate.sh <candidate-carrier.tar.gz>}
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
[ -f "$carrier" ] || { echo "candidate carrier is missing: $carrier" >&2; exit 2; }

stage=$(mktemp -d "${TMPDIR:-/tmp}/aigw-candidate-check.XXXXXX")
trap 'rm -rf "$stage"' EXIT HUP INT TERM

# Reject unsafe archive members before extraction; do not trust tar's default
# extraction behavior for a review artifact.
tar -tzf "$carrier" > "$stage/members"
while IFS= read -r member; do
  case "$member" in
    ''|/*|../*|*/../*|*'/..'|./*)
      echo "candidate carrier contains an unsafe path: $member" >&2
      exit 1
      ;;
  esac
done < "$stage/members"

root_name=$(awk -F/ 'NF {print $1; exit}' "$stage/members")
[ -n "$root_name" ] || { echo "candidate carrier is empty" >&2; exit 1; }
if awk -F/ -v root="$root_name" 'NF && $1 != root {exit 1}' "$stage/members"; then :; else
  echo "candidate carrier must have one root directory" >&2
  exit 1
fi

tar -xzf "$carrier" -C "$stage"
payload="$stage/$root_name"
[ -d "$payload" ] || { echo "candidate payload root is missing" >&2; exit 1; }
[ -f "$payload/candidate.json" ] || { echo "candidate manifest is missing" >&2; exit 1; }
[ -d "$payload/artifacts" ] || { echo "candidate artifacts are missing" >&2; exit 1; }

read_manifest() {
  (cd "$root" && go run -buildvcs=false ./tools/releasekit validate-candidate-manifest "$payload/candidate.json")
}
manifest=$(read_manifest)
version=$(printf '%s\n' "$manifest" | sed -n '1p')
expected_digest=$(printf '%s\n' "$manifest" | sed -n '2p')

if command -v sha256sum >/dev/null 2>&1; then
  actual_digest=$(sha256sum "$payload/artifacts/checksums.txt" | awk '{print tolower($1)}')
else
  actual_digest=$(shasum -a 256 "$payload/artifacts/checksums.txt" | awk '{print tolower($1)}')
fi
[ "$actual_digest" = "$expected_digest" ] || {
  echo "candidate checksum-manifest digest does not match candidate.json" >&2
  exit 1
}

"$root/scripts/checks/release/check-release-artifacts.sh" "$payload/artifacts" "$version" >/dev/null
printf 'verified candidate carrier: OK\n'
