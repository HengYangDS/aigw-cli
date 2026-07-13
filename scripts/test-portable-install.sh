#!/bin/sh
set -eu

source_binary=${1:?usage: test-portable-install.sh <aigw-binary>}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

[ -x "$source_binary" ] || { echo "source binary is not executable: $source_binary" >&2; exit 2; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
home="$tmp/home"
install_dir="$home/bin"
config_file="$home/.config/aigw/config.toml"
profile="$home/.profile"
zshenv="$home/.zshenv"
foreign="$install_dir/foreign-tool"

mkdir -p "$(dirname -- "$config_file")" "$install_dir"
printf 'preserve-me\n' > "$config_file"
printf 'export AIGW_SMOKE_KEEP=1\n' > "$profile"
printf '#!/bin/sh\nexit 0\n' > "$foreign"
chmod 755 "$foreign"

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

env HOME="$home" AIGW_INSTALL_DIR="$install_dir" /bin/sh "$root/scripts/uninstall.sh"

[ ! -e "$installed" ] || { echo "portable uninstall left AIGW binary" >&2; exit 1; }
[ -x "$foreign" ] || { echo "portable uninstall removed a foreign binary" >&2; exit 1; }
[ "$(cat "$config_file")" = "preserve-me" ] || { echo "portable uninstall changed configuration" >&2; exit 1; }
grep -F 'export AIGW_SMOKE_KEEP=1' "$profile" >/dev/null
! grep -F '# >>> AIGW PATH >>>' "$profile" >/dev/null
[ ! -f "$zshenv" ] || ! grep -F '# >>> AIGW PATH bootstrap >>>' "$zshenv" >/dev/null
[ -x "$source_binary" ] || { echo "portable lifecycle changed source binary" >&2; exit 1; }

echo "portable installation lifecycle: OK"
