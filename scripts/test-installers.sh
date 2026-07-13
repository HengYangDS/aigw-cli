#!/bin/sh
set -eu

source_binary=${1:?usage: test-installers.sh <aigw-binary>}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
unix_script="$root/scripts/install.sh"
powershell_script="$root/scripts/install.ps1"

[ -x "$source_binary" ] || {
  echo "source binary is not executable: $source_binary" >&2
  exit 2
}

for forbidden in curl glab GITLAB_TOKEN AIGW_GL_HOST AIGW_PROJECT_URL; do
  if grep -RIn -- "$forbidden" "$unix_script" "$powershell_script" >/dev/null; then
    echo "portable installers must not implement network release retrieval: $forbidden" >&2
    exit 1
  fi
done

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
home="$tmp/home"
install_dir="$home/bin"
profile="$home/.profile"
mkdir -p "$home"

env \
  HOME="$home" \
  SHELL=/bin/sh \
  PATH="/usr/bin:/bin" \
  AIGW_INSTALL_DIR="$install_dir" \
  AIGW_SOURCE_BINARY="$source_binary" \
  /bin/sh "$unix_script"

installed="$install_dir/aigw"
[ -x "$installed" ] || { echo "local-source installer did not produce an executable" >&2; exit 1; }
"$installed" --version >/dev/null
grep -F '# >>> AIGW PATH >>>' "$profile" >/dev/null

archive_dir="$tmp/archive"
mkdir -p "$archive_dir"
cp "$source_binary" "$archive_dir/aigw"
chmod 755 "$archive_dir/aigw"
cp "$unix_script" "$archive_dir/install.sh"
chmod 755 "$archive_dir/install.sh"

archive_home="$tmp/archive-home"
archive_install="$archive_home/bin"
env \
  HOME="$archive_home" \
  SHELL=/bin/sh \
  PATH="/usr/bin:/bin" \
  AIGW_INSTALL_DIR="$archive_install" \
  /bin/sh "$archive_dir/install.sh"
[ -x "$archive_install/aigw" ] || { echo "bundled portable installer did not produce an executable" >&2; exit 1; }
"$archive_install/aigw" --version >/dev/null

set +e
HOME="$tmp/missing-home" SHELL=/bin/sh PATH="/usr/bin:/bin" AIGW_INSTALL_DIR="$tmp/missing-bin" /bin/sh "$unix_script" >"$tmp/missing.out" 2>"$tmp/missing.err"
missing_rc=$?
set -e
[ "$missing_rc" -ne 0 ] || { echo "installer accepted a missing bundled binary" >&2; exit 1; }
grep -F 'only installs a bundled AIGW binary' "$tmp/missing.err" >/dev/null || {
  cat "$tmp/missing.err" >&2
  echo "installer did not explain the local-only boundary" >&2
  exit 1
}

echo "portable installer local-source and bundled-archive contract: OK"
