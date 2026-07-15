#!/bin/sh
set -eu

source_binary=${1:?usage: test-portable-install.sh <aigw-binary>}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

[ -x "$source_binary" ] || { echo "source binary is not executable: $source_binary" >&2; exit 2; }
[ -f "$root/LICENSE" ] || { echo "canonical MIT license is missing" >&2; exit 2; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
home="$tmp/home"
install_dir="$home/bin"
config_file="$home/.config/aigw/config.toml"
profile="$home/.profile"
zshenv="$home/.zshenv"
foreign="$install_dir/foreign-tool"
previous="$tmp/previous-aigw"

mkdir -p "$(dirname -- "$config_file")" "$install_dir"
printf 'preserve-me\n' > "$config_file"
printf 'export AIGW_SMOKE_KEEP=1\n' > "$profile"
printf '#!/bin/sh\nexit 0\n' > "$foreign"
chmod 755 "$foreign"
printf '#!/bin/sh\nprintf "previous AIGW\\n"\n' > "$previous"
chmod 755 "$previous"
cp "$previous" "$install_dir/aigw"

env \
  HOME="$home" \
  PATH="/usr/bin:/bin" \
  SHELL="/bin/sh" \
  AIGW_INSTALL_DIR="$install_dir" \
  AIGW_SOURCE_BINARY="$source_binary" \
  AIGW_VERSION="0.0.0-smoke" \
  /bin/sh "$root/scripts/install.sh"

installed="$install_dir/aigw"
[ -x "$installed" ] || { echo "portable install did not produce an executable" >&2; exit 1; }
"$installed" --version >/dev/null
grep -F '# >>> AIGW PATH >>>' "$profile" >/dev/null
cmp "$previous" "$install_dir/.aigw.previous" >/dev/null || {
  echo "portable install did not retain the immediately preceding AIGW binary" >&2
  exit 1
}
[ -x "$install_dir/.aigw.previous" ] || { echo "portable install backup is not executable" >&2; exit 1; }

set +e
env -i HOME="$home" PATH="" AIGW_INSTALL_DIR="$install_dir" /bin/sh "$root/scripts/uninstall.sh" --help >"$tmp/uninstall-help.out" 2>"$tmp/uninstall-help.err"
uninstall_help_rc=$?
set -e
[ "$uninstall_help_rc" -eq 0 ] || { cat "$tmp/uninstall-help.err" >&2; echo "uninstaller help exited unsuccessfully" >&2; exit 1; }
grep -F 'Usage: uninstall.sh' "$tmp/uninstall-help.out" >/dev/null || {
  cat "$tmp/uninstall-help.out" >&2
  echo "uninstaller help did not print usage" >&2
  exit 1
}
[ -x "$installed" ] || { echo "uninstaller help removed the AIGW binary" >&2; exit 1; }
grep -F '# >>> AIGW PATH >>>' "$profile" >/dev/null || { echo "uninstaller help changed shell configuration" >&2; exit 1; }

env -i HOME="$home" PATH="" AIGW_INSTALL_DIR="$install_dir" /bin/sh "$root/scripts/uninstall.sh"

[ ! -e "$installed" ] || { echo "portable uninstall left AIGW binary" >&2; exit 1; }
[ ! -e "$install_dir/.aigw.previous" ] || { echo "portable uninstall left AIGW rollback binary" >&2; exit 1; }
[ -x "$foreign" ] || { echo "portable uninstall removed a foreign binary" >&2; exit 1; }
[ "$(cat "$config_file")" = "preserve-me" ] || { echo "portable uninstall changed configuration" >&2; exit 1; }
grep -F 'export AIGW_SMOKE_KEEP=1' "$profile" >/dev/null
! grep -F '# >>> AIGW PATH >>>' "$profile" >/dev/null
[ ! -f "$zshenv" ] || ! grep -F '# >>> AIGW PATH bootstrap >>>' "$zshenv" >/dev/null
[ -x "$source_binary" ] || { echo "portable lifecycle changed source binary" >&2; exit 1; }
[ -f "$root/LICENSE" ] || { echo "portable lifecycle changed the canonical MIT license" >&2; exit 1; }

echo "portable installation lifecycle: OK"
