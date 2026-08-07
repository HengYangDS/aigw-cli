#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

# Exercise the script's own dependency list without relying on the host's
# installed build tools. Every required command is represented by a no-op
# fixture so one intentionally missing command is the only failure cause.
for command in \
  go lipo pkgbuild productbuild nfpm wixl msibuild xar \
  file tar zip unzip ar bsdtar msiextract msiinfo pkgutil shasum sha256sum
do
  cat > "$tmp/$command" <<'SH'
#!/bin/sh
exit 0
SH
  chmod 755 "$tmp/$command"
done

PATH="$tmp:/usr/bin:/bin" sh "$root/scripts/checks/release/check-package-runner.sh" > "$tmp/ok.out"
grep -Fx 'package runner capability: OK' "$tmp/ok.out" >/dev/null || {
  cat "$tmp/ok.out" >&2
  echo "package-runner preflight did not report success" >&2
  exit 1
}

rm "$tmp/msiinfo"
if PATH="$tmp:/usr/bin:/bin" sh "$root/scripts/checks/release/check-package-runner.sh" > "$tmp/missing.out" 2>&1; then
  cat "$tmp/missing.out" >&2
  echo "package-runner preflight accepted a missing validator" >&2
  exit 1
fi
grep -Fx 'missing package-runner command: msiinfo' "$tmp/missing.out" >/dev/null || {
  cat "$tmp/missing.out" >&2
  echo "package-runner preflight did not name the missing validator" >&2
  exit 1
}

cat > "$tmp/msiinfo" <<'SH'
#!/bin/sh
exit 0
SH
chmod 755 "$tmp/msiinfo"

rm "$tmp/xar"
if PATH="$tmp" /bin/sh "$root/scripts/checks/release/check-package-runner.sh" > "$tmp/xar.out" 2>&1; then
  cat "$tmp/xar.out" >&2
  echo "package-runner preflight accepted a missing XAR normalizer" >&2
  exit 1
fi
grep -Fx 'missing package-runner command: xar' "$tmp/xar.out" >/dev/null || {
  cat "$tmp/xar.out" >&2
  echo "package-runner preflight did not name the missing XAR normalizer" >&2
  exit 1
}

cat > "$tmp/xar" <<'SH'
#!/bin/sh
exit 0
SH
chmod 755 "$tmp/xar"

rm -f "$tmp/sha256sum" "$tmp/shasum"
# /bin can be a symlink to /usr/bin on Linux runners, so it is not a valid
# absence fixture. Invoke the known shell by its absolute path while exposing
# only the controlled fixture directory through PATH.
if PATH="$tmp" /bin/sh "$root/scripts/checks/release/check-package-runner.sh" > "$tmp/checksum.out" 2>&1; then
  cat "$tmp/checksum.out" >&2
  echo "package-runner preflight accepted no SHA-256 utility" >&2
  exit 1
fi
grep -Fx 'missing package-runner SHA-256 utility: sha256sum or shasum' "$tmp/checksum.out" >/dev/null || {
  cat "$tmp/checksum.out" >&2
  echo "package-runner preflight did not name the missing SHA-256 capability" >&2
  exit 1
}

echo "package runner capability contract: OK"
