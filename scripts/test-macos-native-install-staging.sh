#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
acceptance="$root/scripts/test-macos-native-install.sh"
uninstaller="$root/packaging/macos/aigw-uninstall"

[ -x "$acceptance" ] || {
  echo "macOS native-install acceptance script is missing or not executable" >&2
  exit 1
}
[ -x "$uninstaller" ] || {
  echo "macOS package uninstaller is missing or not executable" >&2
  exit 1
}

sh -n "$acceptance"
sh -n "$uninstaller"

require() {
  token=$1
  file=$2
  grep -Fq -- "$token" "$file" || {
    echo "macOS native acceptance contract is missing $token in $file" >&2
    exit 1
  }
}

for token in \
  'AIGW_MACOS_ACCEPTANCE_USER' \
  'AIGW_MACOS_UPGRADE_PACKAGE' \
  'upgrade baseline must differ from the candidate version' \
  'run_as_acceptance_user' \
  'sudo -u "$acceptance_user"' \
  'hdiutil create' \
  'hdiutil attach' \
  'installer -verboseR -pkg' \
  'pkgutil --volume' \
  'usr/local/libexec/aigw/uninstall' \
  'must run as root' \
  'macOS native package lifecycle: OK'
do
  require "$token" "$acceptance"
done

for token in \
  'dig.aigw.cli' \
  'usr/local/bin/aigw' \
  'usr/local/libexec/aigw/uninstall' \
  'pkgutil --volume' \
  'pkgutil --volume "$target" --forget dig.aigw.cli' \
  'must run as root'
do
  require "$token" "$uninstaller"
done

if grep -nE '/Users/|AIGW_MACOS_ACCEPTANCE_USER=[^$]' "$acceptance" "$uninstaller"; then
  echo "macOS native acceptance must not hard-code a host account or home path" >&2
  exit 1
fi

echo "macOS native-install acceptance contract: OK"
