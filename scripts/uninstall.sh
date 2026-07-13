#!/bin/sh
set -eu

# Portable uninstall must work even when invoked from a restricted shell. It
# removes only AIGW-owned files and marked blocks, so use the same trusted
# system-tool bootstrap as portable install rather than inheriting PATH.
PATH=/usr/bin:/bin:/usr/sbin:/sbin
export PATH

install_dir=${AIGW_INSTALL_DIR:-"$HOME/.local/bin"}
binary="$install_dir/aigw"
if [ -n "${AIGW_SHIM_DIR:-}" ]; then
  shim_dir="$AIGW_SHIM_DIR"
else
  case "$(uname -s)" in
    Darwin) shim_dir="$HOME/Library/Application Support/aigw/bin" ;;
    Linux) shim_dir="${XDG_DATA_HOME:-$HOME/.local/share}/aigw/bin" ;;
    *) shim_dir="$HOME/.local/share/aigw/bin" ;;
  esac
fi
shim="$shim_dir/claude"

if [ -f "$shim" ]; then
  if grep -q 'AIGW managed Claude shim' "$shim"; then
    rm -f "$shim"
  else
    echo "Refusing to remove non-AIGW Claude launcher: $shim" >&2
    exit 1
  fi
fi
rm -f "$binary"

for profile in "$HOME/.zshenv" "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.bashrc" "$HOME/.profile" "$HOME/.config/fish/config.fish"; do
  [ -f "$profile" ] || continue
  tmp="$profile.aigw.$$"
  awk '
    $0 == "# >>> AIGW PATH >>>" {skip=1; next}
    $0 == "# <<< AIGW PATH <<<" {skip=0; next}
    $0 == "# >>> AIGW Claude shim PATH >>>" {skip=1; next}
    $0 == "# <<< AIGW Claude shim PATH <<<" {skip=0; next}
    $0 == "# >>> AIGW PATH bootstrap >>>" {skip=1; next}
    $0 == "# <<< AIGW PATH bootstrap <<<" {skip=0; next}
    skip != 1 {print}
  ' "$profile" > "$tmp"
  mv "$tmp" "$profile"
done

echo "Removed AIGW executable, owned Claude launcher, and AIGW PATH blocks. Configuration and system-keyring secrets were preserved."
