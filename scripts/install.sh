#!/bin/sh
set -eu

# The portable installer is invoked from arbitrary shell environments. Start
# from the platform's trusted base tool directories instead of inheriting a
# user-modified or empty PATH; the installed AIGW PATH block is handled below.
PATH=/usr/bin:/bin:/usr/sbin:/sbin
export PATH

install_dir=${AIGW_INSTALL_DIR:-"$HOME/.local/bin"}
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

shell_profile() {
  shell_name=$(basename "${SHELL:-sh}")
  case "$shell_name" in
    zsh) echo "$HOME/.zshrc" ;;
    bash) [ -f "$HOME/.bash_profile" ] && echo "$HOME/.bash_profile" || echo "$HOME/.bashrc" ;;
    fish) mkdir -p "$HOME/.config/fish"; echo "$HOME/.config/fish/config.fish" ;;
    *) echo "$HOME/.profile" ;;
  esac
}

ensure_path() {
  case ":$PATH:" in *":$install_dir:"*) return 0 ;; esac
  profile=$(shell_profile)
  mkdir -p "$(dirname -- "$profile")"
  begin="# >>> AIGW PATH >>>"
  end="# <<< AIGW PATH <<<"
  tmp="$profile.aigw.$$"
  if [ -f "$profile" ]; then
    awk -v begin="$begin" -v end="$end" '
      $0 == begin {skip=1; next}
      $0 == end {skip=0; next}
      skip != 1 {print}
    ' "$profile" > "$tmp"
  else
    : > "$tmp"
  fi
  shell_name=$(basename "${SHELL:-sh}")
  {
    printf '\n%s\n' "$begin"
    if [ "$shell_name" = fish ]; then
      printf 'fish_add_path %s\n' "$install_dir"
    else
      printf 'export PATH="%s:$PATH"\n' "$install_dir"
    fi
    printf '%s\n' "$end"
  } >> "$tmp"
  mv "$tmp" "$profile"
  echo "PATH updated in $profile. Open a new terminal, or run: export PATH=\"$install_dir:\$PATH\""
}

# AIGW_SOURCE_BINARY is a controlled local test/repair seam. A portable
# release archive supplies the sibling aigw binary. Network retrieval belongs
# exclusively to `aigw update`, which owns version, checksum, timeout, and
# redirect policy in one auditable implementation.
if [ -n "${AIGW_SOURCE_BINARY:-}" ]; then
  [ -f "$AIGW_SOURCE_BINARY" ] && [ -x "$AIGW_SOURCE_BINARY" ] || {
    echo "AIGW_SOURCE_BINARY must reference an executable local file" >&2
    exit 2
  }
  source_binary=$AIGW_SOURCE_BINARY
elif [ -x "$script_dir/aigw" ]; then
  source_binary="$script_dir/aigw"
else
  echo "This installer only installs a bundled AIGW binary. Download and extract the matching portable archive first; use \`aigw update\` after installation." >&2
  exit 2
fi

mkdir -p "$install_dir"
tmp_binary="$install_dir/.aigw.new.$$"
cp "$source_binary" "$tmp_binary"
chmod 755 "$tmp_binary"
mv "$tmp_binary" "$install_dir/aigw"
ensure_path

echo "Installed $install_dir/aigw"
echo "Next: aigw setup"
