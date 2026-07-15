#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
artifacts=${1:?usage: test-macos-native-install.sh <artifact-directory> <version>}
version=${2:?usage: test-macos-native-install.sh <artifact-directory> <version>}
acceptance_user=${AIGW_MACOS_ACCEPTANCE_USER:?AIGW_MACOS_ACCEPTANCE_USER is required}
upgrade_package=${AIGW_MACOS_UPGRADE_PACKAGE:?AIGW_MACOS_UPGRADE_PACKAGE is required}

if [ "$(id -u)" -ne 0 ]; then
  echo "macOS native package acceptance must run as root" >&2
  exit 2
fi

case "$acceptance_user" in
  *[!A-Za-z0-9_-]*|'')
    echo "AIGW_MACOS_ACCEPTANCE_USER must be a local account short name" >&2
    exit 2
    ;;
esac

for command in hdiutil installer pkgutil su; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "required for macOS native-install acceptance: $command" >&2
    exit 2
  }
done

[ -d "$artifacts" ] || { echo "artifact directory does not exist: $artifacts" >&2; exit 2; }
[ -f "$upgrade_package" ] || { echo "upgrade package does not exist: $upgrade_package" >&2; exit 2; }
sh "$root/scripts/check-release-artifacts.sh" "$artifacts" "$version" >/dev/null

package="$artifacts/aigw_${version}_darwin_universal.pkg"
candidate_stage=$(mktemp -d /tmp/aigw-macos-native-candidate.XXXXXX)
upgrade_stage=$(mktemp -d /tmp/aigw-macos-native-upgrade.XXXXXX)
user_stage=$(mktemp -d /tmp/aigw-macos-native-user.XXXXXX)
candidate_image="$candidate_stage/target.dmg"
upgrade_image="$upgrade_stage/target.dmg"
candidate_mount="$candidate_stage/mount"
upgrade_mount="$upgrade_stage/mount"
user_home="$user_stage/home"
candidate_device=''
upgrade_device=''

cleanup() {
  if [ -n "$upgrade_device" ]; then hdiutil detach "$upgrade_device" -force >/dev/null 2>&1 || true; fi
  if [ -n "$candidate_device" ]; then hdiutil detach "$candidate_device" -force >/dev/null 2>&1 || true; fi
  rm -rf "$candidate_stage" "$upgrade_stage" "$user_stage"
}
trap cleanup EXIT HUP INT TERM

attach_image() {
  image=$1
  mount=$2
  output=$(hdiutil attach -nobrowse -noverify -mountpoint "$mount" "$image")
  device=$(printf '%s\n' "$output" | awk '/^\/dev\// {print $1; exit}')
  [ -n "$device" ] || { echo "could not determine mounted APFS device" >&2; exit 1; }
  printf '%s\n' "$device"
}

assert_installed() {
  mount=$1
  expected=$2
  binary="$mount/usr/local/bin/aigw"
  [ -x "$binary" ] || { echo "macOS package did not install /usr/local/bin/aigw" >&2; exit 1; }
  "$binary" --version | grep -Fx "aigw version $expected" >/dev/null
  "$binary" --help >/dev/null
  [ -x "$mount/usr/local/libexec/aigw/uninstall" ] || {
    echo "macOS package did not install its owned uninstaller" >&2
    exit 1
  }
  pkgutil --volume "$mount" --pkg-info dig.aigw.cli >/dev/null
}

prepare_isolated_user_home() {
  user_id=$(id -u "$acceptance_user" 2>/dev/null) || {
    echo "AIGW_MACOS_ACCEPTANCE_USER does not name a local account" >&2
    exit 2
  }
  group_id=$(id -g "$acceptance_user" 2>/dev/null) || {
    echo "cannot determine the primary group for AIGW_MACOS_ACCEPTANCE_USER" >&2
    exit 2
  }
  mkdir -p "$user_home"
  chown "$user_id:$group_id" "$user_stage" "$user_home"
  chmod 755 "$user_stage"
  chmod 700 "$user_home"
}

run_as_acceptance_user() {
  target_home=$1
  binary=$2
  command -v sudo >/dev/null 2>&1 || {
    echo "required for isolated-user macOS acceptance: sudo" >&2
    return 2
  }
  sudo -u "$acceptance_user" env HOME="$target_home" AIGW_SECRET_BACKEND=env \
    "$binary" status --json >/dev/null
}

printf '%s\n' 'creating isolated macOS installation target'
mkdir -p "$candidate_mount" "$upgrade_mount"
hdiutil create -quiet -size 256m -fs APFS -volname AIGWCandidate -ov -type UDIF "$candidate_image"
candidate_device=$(attach_image "$candidate_image" "$candidate_mount")
installer -verboseR -pkg "$package" -target "$candidate_mount"
assert_installed "$candidate_mount" "$version"

# A separate image proves that the same artifact works on a clean target. The
# caller provides a distinct, previously verified package for the upgrade path.
hdiutil create -quiet -size 256m -fs APFS -volname AIGWUpgrade -ov -type UDIF "$upgrade_image"
upgrade_device=$(attach_image "$upgrade_image" "$upgrade_mount")
installer -verboseR -pkg "$upgrade_package" -target "$upgrade_mount"
old_version=$("$upgrade_mount/usr/local/bin/aigw" --version | sed 's/^aigw version //')
[ -n "$old_version" ] || { echo "upgrade baseline did not expose an AIGW version" >&2; exit 1; }
[ "$old_version" != "$version" ] || { echo "upgrade baseline must differ from the candidate version" >&2; exit 2; }
installer -verboseR -pkg "$package" -target "$upgrade_mount"
assert_installed "$upgrade_mount" "$version"

prepare_isolated_user_home
run_as_acceptance_user "$user_home" "$candidate_mount/usr/local/bin/aigw" || {
  echo "installed AIGW could not run as the isolated acceptance user" >&2
  exit 1
}

"$candidate_mount/usr/local/libexec/aigw/uninstall" "$candidate_mount"
[ ! -e "$candidate_mount/usr/local/bin/aigw" ] || { echo "native uninstall left AIGW binary" >&2; exit 1; }
[ ! -e "$candidate_mount/usr/local/libexec/aigw/uninstall" ] || { echo "native uninstall left owned uninstaller" >&2; exit 1; }
if pkgutil --volume "$candidate_mount" --pkg-info dig.aigw.cli >/dev/null 2>&1; then
  echo "native uninstall left an AIGW package receipt" >&2
  exit 1
fi

echo "macOS native package lifecycle: OK"
