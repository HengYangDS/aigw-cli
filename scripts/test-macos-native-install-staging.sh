#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
acceptance="$root/scripts/test-macos-native-install.sh"
uninstaller="$root/packaging/macos/aigw-uninstall"

[ -x "$acceptance" ] || {
  echo "macOS native-install acceptance script is missing or not executable" >&2
  exit 1
}
[ -x "$uninstaller" ] || {
  echo "macOS package uninstaller is missing or not executable" >&2
  exit 1
}

sh -n "$acceptance"
sh -n "$uninstaller"

require() {
  token=$1
  file=$2
  grep -Fq -- "$token" "$file" || {
    echo "macOS native acceptance contract is missing $token in $file" >&2
    exit 1
  }
}

for token in \
  'AIGW_MACOS_ACCEPTANCE_USER' \
  'AIGW_MACOS_UPGRADE_PACKAGE' \
  'upgrade baseline must differ from the candidate version' \
  'run_as_acceptance_user' \
  'sudo -u "$acceptance_user"' \
  'hdiutil create' \
  'hdiutil attach' \
  'installer -verboseR -pkg' \
  'pkgutil --volume' \
  'usr/local/libexec/aigw/uninstall' \
  'must run as root' \
  'macOS native package lifecycle: OK'
do
  require "$token" "$acceptance"
done

for token in \
  'dig.aigw.cli' \
  'usr/local/bin/aigw' \
  'usr/local/libexec/aigw/uninstall' \
  'pkgutil --volume' \
  'pkgutil --volume "$target" --forget dig.aigw.cli' \
  'pkgutil --forget dig.aigw.cli' \
  'must run as root'
do
  require "$token" "$uninstaller"
done

if grep -nE '/Users/|AIGW_MACOS_ACCEPTANCE_USER=[^$]' "$acceptance" "$uninstaller"; then
  echo "macOS native acceptance must not hard-code a host account or home path" >&2
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
bin="$tmp/bin"
volume="$tmp/volume"
log="$tmp/commands"
mkdir -p "$bin" "$volume/usr/local/bin" "$volume/usr/local/libexec/aigw"
printf 'candidate\n' > "$volume/usr/local/bin/aigw"
printf 'uninstaller\n' > "$volume/usr/local/libexec/aigw/uninstall"

cat > "$bin/id" <<'SH'
#!/bin/sh
[ "$1" = -u ] || exit 2
printf '0\n'
SH
cat > "$bin/pkgutil" <<'SH'
#!/bin/sh
{
  printf 'pkgutil'
  for value in "$@"; do printf ' <%s>' "$value"; done
  printf '\n'
} >> "$AIGW_TEST_PKGUTIL_LOG"
SH
cat > "$bin/rm" <<'SH'
#!/bin/sh
exec /bin/rm "$@"
SH
cat > "$bin/rmdir" <<'SH'
#!/bin/sh
exec /bin/rmdir "$@"
SH
chmod 755 "$bin"/*

AIGW_TEST_PKGUTIL_LOG="$log" PATH="$bin:/usr/bin:/bin" "$uninstaller" "$volume" >/dev/null 2>&1
[ ! -e "$volume/usr/local/bin/aigw" ] || {
  echo "macOS uninstaller left the package-owned binary" >&2
  exit 1
}
[ ! -e "$volume/usr/local/libexec/aigw/uninstall" ] || {
  echo "macOS uninstaller left its package-owned helper" >&2
  exit 1
}
grep -Fx "pkgutil <--volume> <$volume> <--forget> <dig.aigw.cli>" "$log" >/dev/null || {
  cat "$log" >&2
  echo "macOS uninstaller did not forget the disposable-volume receipt" >&2
  exit 1
}

if AIGW_TEST_PKGUTIL_LOG="$tmp/relative-pkgutil.out" PATH="$bin:/usr/bin:/bin" "$uninstaller" relative > "$tmp/relative.out" 2>&1; then
  echo "macOS uninstaller accepted a relative target" >&2
  exit 1
fi
grep -F 'absolute volume path' "$tmp/relative.out" >/dev/null || {
  cat "$tmp/relative.out" >&2
  echo "macOS uninstaller did not explain the relative-target rejection" >&2
  exit 1
}

AIGW_TEST_PKGUTIL_LOG="$tmp/root.out" PATH="$bin:/usr/bin:/bin" "$uninstaller" / >/dev/null 2>&1
grep -Fx 'pkgutil <--forget> <dig.aigw.cli>' "$tmp/root.out" >/dev/null || {
  cat "$tmp/root.out" >&2
  echo "macOS uninstaller did not forget the root-volume receipt" >&2
  exit 1
}

echo "macOS native-install acceptance contract: OK"
