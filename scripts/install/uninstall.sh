#!/bin/sh
set -eu

# Portable uninstall must work even when invoked from a restricted shell. It
# removes only AIGW-owned files and marked blocks, so use the same trusted
# system-tool bootstrap as portable install rather than inheriting PATH.
PATH=/usr/bin:/bin:/usr/sbin:/sbin
export PATH

usage() {
  cat <<'EOF'
Usage: uninstall.sh [--help]

Remove only files and shell configuration blocks owned by AIGW.
EOF
}

case "${1:-}" in
  '') ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac

install_dir=${AIGW_INSTALL_DIR:-"$HOME/.local/bin"}
binary="$install_dir/aigw"
backup="$install_dir/.aigw.previous"
if [ -n "${AIGW_LAUNCHER_DIR:-}" ]; then
  launcher_dir="$AIGW_LAUNCHER_DIR"
else
  case "$(uname -s)" in
    Darwin) launcher_dir="$HOME/Library/Application Support/aigw/bin" ;;
    Linux) launcher_dir="${XDG_DATA_HOME:-$HOME/.local/share}/aigw/bin" ;;
    *) launcher_dir="$HOME/.local/share/aigw/bin" ;;
  esac
fi
launcher="$launcher_dir/claude"

if [ -f "$launcher" ]; then
  if grep -q 'AIGW managed Claude launcher' "$launcher"; then
    rm -f "$launcher"
  else
    echo "Refusing to remove non-AIGW Claude launcher: $launcher" >&2
    exit 1
  fi
fi
rm -f "$binary" "$backup"

for profile in "$HOME/.zshenv" "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.bashrc" "$HOME/.profile" "$HOME/.config/fish/config.fish"; do
  [ -f "$profile" ] || continue
  tmp="$profile.aigw.$$"
  awk '
    $0 == "# >>> AIGW PATH >>>" {skip=1; next}
    $0 == "# <<< AIGW PATH <<<" {skip=0; next}
    $0 == "# >>> AIGW Claude launcher PATH >>>" {skip=1; next}
    $0 == "# <<< AIGW Claude launcher PATH <<<" {skip=0; next}
    $0 == "# >>> AIGW PATH bootstrap >>>" {skip=1; next}
    $0 == "# <<< AIGW PATH bootstrap <<<" {skip=0; next}
    skip != 1 {print}
  ' "$profile" > "$tmp"
  mv "$tmp" "$profile"
done

echo "Removed AIGW executable, owned Claude launcher, and AIGW PATH blocks. Configuration and system-keyring secrets were preserved."
