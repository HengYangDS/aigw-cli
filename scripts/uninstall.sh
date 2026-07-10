#!/bin/sh
set -eu

install_dir=${AIGW_INSTALL_DIR:-"$HOME/.local/bin"}
binary="$install_dir/aigw"
shim="$install_dir/claude"

if [ -f "$shim" ]; then
  if grep -q 'AIGW managed Claude shim' "$shim"; then
    rm -f "$shim"
  else
    echo "Refusing to remove non-AIGW Claude launcher: $shim" >&2
    exit 1
  fi
fi
rm -f "$binary"

for profile in "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.bashrc" "$HOME/.profile" "$HOME/.config/fish/config.fish"; do
  [ -f "$profile" ] || continue
  tmp="$profile.aigw.$$"
  awk '
    $0 == "# >>> AIGW PATH >>>" {skip=1; next}
    $0 == "# <<< AIGW PATH <<<" {skip=0; next}
    skip != 1 {print}
  ' "$profile" > "$tmp"
  mv "$tmp" "$profile"
done

echo "Removed AIGW executable, owned launcher, and AIGW PATH block. Configuration and system-keyring secrets were preserved."
