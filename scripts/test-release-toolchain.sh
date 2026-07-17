#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-release-toolchain.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

expected=$(awk '$1 == "go" { print $2; exit }' "$root/go.mod")
cat > "$tmp/go" <<EOF
#!/bin/sh
set -eu
[ "\$1" = env ] && [ "\$2" = GOVERSION ] || exit 2
printf '%s\\n' 'go$expected'
EOF
chmod 755 "$tmp/go"
PATH="$tmp:/usr/bin:/bin" sh "$checker" > "$tmp/ok.out"
grep -Fx "release toolchain: go$expected OK" "$tmp/ok.out" >/dev/null

cat > "$tmp/go" <<'EOF'
#!/bin/sh
set -eu
[ "$1" = env ] && [ "$2" = GOVERSION ] || exit 2
printf '%s\n' go0.0.0
EOF
chmod 755 "$tmp/go"
if PATH="$tmp:/usr/bin:/bin" sh "$checker" > "$tmp/bad.out" 2>&1; then
  echo "release toolchain checker accepted the wrong compiler" >&2
  exit 1
fi
grep -Fx "release toolchain: expected go$expected, found go0.0.0" "$tmp/bad.out" >/dev/null

echo "release toolchain contract: OK"
