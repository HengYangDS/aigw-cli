#!/bin/sh
set -eu

# The portable installer is invoked from arbitrary shell environments. Start
# from the platform's trusted base tool directories instead of inheriting a
# user-modified or empty PATH; the installed AIGW PATH block is handled below.
initial_path=${PATH-}
PATH=/usr/bin:/bin:/usr/sbin:/sbin
export PATH

usage() {
  cat <<'EOF'
Usage: install.sh [--help]

Install the bundled AIGW binary for the current user.
EOF
}

case "${1:-}" in
  '') ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac

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

owned_block_matches() {
  file=$1
  begin=$2
  end=$3
  expected=$4
  [ -f "$file" ] || return 1
  [ "$(grep -Fxc "$begin" "$file" || true)" = 1 ] || return 1
  [ "$(grep -Fxc "$end" "$file" || true)" = 1 ] || return 1
  actual=$(awk -v begin="$begin" -v end="$end" '
    $0 == begin {inside=1}
    inside {print}
    inside && $0 == end {exit}
  ' "$file")
  [ "$actual" = "$expected" ]
}

ensure_zsh_bootstrap() {
  [ "$(basename "${SHELL:-sh}")" = zsh ] || return 0
  case ":$initial_path:" in *:/usr/bin:*|*:/bin:*) return 0 ;; esac
  bootstrap="$HOME/.zshenv"
  begin="# >>> AIGW PATH bootstrap >>>"
  end="# <<< AIGW PATH bootstrap <<<"
  line='case ":$PATH:" in *:/usr/bin:*|*:/bin:*) ;; *) export PATH="/usr/bin:/bin:/usr/sbin:/sbin:$PATH" ;; esac'
  expected=$(printf '%s\n%s\n%s' "$begin" "$line" "$end")
  owned_block_matches "$bootstrap" "$begin" "$end" "$expected" && return 0
  tmp="$bootstrap.aigw.$$"
  if [ -f "$bootstrap" ]; then
    awk -v begin="$begin" -v end="$end" '
      $0 == begin {skip=1; next}
      $0 == end {skip=0; next}
      skip != 1 {print}
    ' "$bootstrap" > "$tmp"
  else
    : > "$tmp"
  fi
  {
    printf '\n%s\n' "$begin"
    printf '%s\n' "$line"
    printf '%s\n' "$end"
  } >> "$tmp"
  mv "$tmp" "$bootstrap"
}

ensure_path() {
  profile=$(shell_profile)
  begin="# >>> AIGW PATH >>>"
  end="# <<< AIGW PATH <<<"
  shell_name=$(basename "${SHELL:-sh}")
  if [ "$shell_name" = fish ]; then
    line="fish_add_path $install_dir /usr/bin /bin /usr/sbin /sbin"
  else
    line="export PATH=\"$install_dir:/usr/bin:/bin:/usr/sbin:/sbin:\$PATH\""
  fi
  expected=$(printf '%s\n%s\n%s' "$begin" "$line" "$end")
  owned_block_matches "$profile" "$begin" "$end" "$expected" && return 0
  mkdir -p "$(dirname -- "$profile")"
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
  {
    printf '\n%s\n' "$begin"
    printf '%s\n' "$line"
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
binary="$install_dir/aigw"
backup="$install_dir/.aigw.previous"
had_previous=0
if [ -f "$binary" ]; then
  tmp_backup="$install_dir/.aigw.previous.new.$$"
  cp "$binary" "$tmp_backup"
  chmod 755 "$tmp_backup"
  mv "$tmp_backup" "$backup"
  had_previous=1
fi
tmp_binary="$install_dir/.aigw.new.$$"
cp "$source_binary" "$tmp_binary"
chmod 755 "$tmp_binary"
mv "$tmp_binary" "$binary"
ensure_zsh_bootstrap
ensure_path

echo "Installed $binary"
if [ "$had_previous" -eq 0 ]; then
  echo "Next: aigw setup"
else
  echo "Previous AIGW binary saved to $backup"
  echo "Next: aigw check"
fi
