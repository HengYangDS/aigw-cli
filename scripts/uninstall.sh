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
echo "Removed AIGW executable and owned launcher. Configuration and system-keyring secrets were preserved."
