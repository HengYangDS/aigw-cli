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
  PATH="" \
  AIGW_INSTALL_DIR="$install_dir" \
  AIGW_SOURCE_BINARY="$source_binary" \
  /bin/sh "$unix_script"

installed="$install_dir/aigw"
[ -x "$installed" ] || { echo "local-source installer did not produce an executable" >&2; exit 1; }
"$installed" --version >/dev/null
grep -F '# >>> AIGW PATH >>>' "$profile" >/dev/null
grep -F 'export PATH="' "$profile" | grep -F '/usr/bin:/bin:/usr/sbin:/sbin' >/dev/null
env -i HOME="$home" SHELL=/bin/sh PATH="" /bin/sh -c '. "$HOME/.profile"; command -v aigw; command -v mkdir; command -v mv' >"$tmp/activated-path.out"
grep -Fx "$install_dir/aigw" "$tmp/activated-path.out" >/dev/null
for tool in mkdir mv; do
  resolved=$(grep -F "/$tool" "$tmp/activated-path.out" | head -n 1 || true)
  case "$resolved" in /usr/bin/"$tool"|/bin/"$tool") ;; *) echo "activated PATH did not restore $tool: $resolved" >&2; exit 1 ;; esac
done

zsh_home="$tmp/zsh-home"
mkdir -p "$zsh_home"
env HOME="$zsh_home" SHELL=/bin/zsh PATH="" AIGW_INSTALL_DIR="$zsh_home/bin" AIGW_SOURCE_BINARY="$source_binary" /bin/sh "$unix_script"
set +e
env -i HOME="$zsh_home" PATH="" SHELL=/bin/zsh /bin/zsh -ic 'command -v aigw; command -v locale; command -v mkdir; command -v mv; aigw --version' >"$tmp/zsh-activated.out" 2>"$tmp/zsh-activated.err"
zsh_rc=$?
set -e
[ "$zsh_rc" -eq 0 ] || { cat "$tmp/zsh-activated.err" >&2; echo "zsh activation failed with empty PATH" >&2; exit 1; }
[ ! -s "$tmp/zsh-activated.err" ] || { cat "$tmp/zsh-activated.err" >&2; echo "zsh emitted startup errors after AIGW install" >&2; exit 1; }
grep -Fx "$zsh_home/bin/aigw" "$tmp/zsh-activated.out" >/dev/null
for tool in locale mkdir mv; do
  grep -E "/$tool$" "$tmp/zsh-activated.out" >/dev/null || { echo "zsh activation did not restore $tool" >&2; exit 1; }
done

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
  PATH="" \
  AIGW_INSTALL_DIR="$archive_install" \
  /bin/sh "$archive_dir/install.sh"
[ -x "$archive_install/aigw" ] || { echo "bundled portable installer did not produce an executable" >&2; exit 1; }
"$archive_install/aigw" --version >/dev/null

help_home="$tmp/help-home"
help_install="$help_home/bin"
set +e
env \
  HOME="$help_home" \
  SHELL=/bin/sh \
  PATH="" \
  AIGW_INSTALL_DIR="$help_install" \
  AIGW_SOURCE_BINARY="$source_binary" \
  /bin/sh "$unix_script" --help >"$tmp/help.out" 2>"$tmp/help.err"
help_rc=$?
set -e
[ "$help_rc" -eq 0 ] || { cat "$tmp/help.err" >&2; echo "installer help exited unsuccessfully" >&2; exit 1; }
grep -F 'Usage: install.sh' "$tmp/help.out" >/dev/null || {
  cat "$tmp/help.out" >&2
  echo "installer help did not print usage" >&2
  exit 1
}
[ ! -e "$help_install/aigw" ] || { echo "installer help modified the installation" >&2; exit 1; }

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
