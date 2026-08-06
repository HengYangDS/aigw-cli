#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
checker="$root/scripts/checks/governance/check-portability.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

new_fixture() {
  name=$1
  repo="$tmp/$name"
  mkdir -p "$repo"
  git -C "$repo" init -q
  printf '%s\n' "$repo"
}

expect_rejected() {
  label=$1
  repo=$2
  if "$checker" "$repo" >"$tmp/$label.out" 2>&1; then
    echo "portability checker accepted $label production hard-coding" >&2
    exit 1
  fi
}

production_go=$(new_fixture production-go)
mkdir -p "$production_go/internal/core"
cat >"$production_go/internal/core/config.go" <<'EOF'
package core

const configPath = "/Users/example/.config/product/config.toml"
EOF
git -C "$production_go" add .
expect_rejected production-go "$production_go"
grep -F 'internal/core/config.go:3: absolute user-home path' "$tmp/production-go.out" >/dev/null

untracked_go=$(new_fixture untracked-go)
mkdir -p "$untracked_go/internal/core"
cat >"$untracked_go/internal/core/config.go" <<'EOF'
package core

const configPath = "/Users/example/.config/product/config.toml"
EOF
expect_rejected untracked-go "$untracked_go"
grep -F 'internal/core/config.go:3: absolute user-home path' "$tmp/untracked-go.out" >/dev/null

missing_file=$(new_fixture missing-file)
mkdir -p "$missing_file/internal/core"
printf '%s\n' 'package core' >"$missing_file/internal/core/config.go"
git -C "$missing_file" add .
rm "$missing_file/internal/core/config.go"
"$checker" "$missing_file" >/dev/null

symlink_file=$(new_fixture symlink-file)
mkdir -p "$symlink_file/internal/core"
printf '%s\n' 'portable' >"$symlink_file/target.txt"
ln -s ../../target.txt "$symlink_file/internal/core/config.go"
git -C "$symlink_file" add .
expect_rejected symlink-file "$symlink_file"
grep -F 'candidate_symlink:internal/core/config.go' "$tmp/symlink-file.out" >/dev/null

production_tool=$(new_fixture production-tool)
mkdir -p "$production_tool/tools/check"
cat >"$production_tool/tools/check/main.go" <<'EOF'
package main

const configPath = `C:\Users\example\product\config.toml`
EOF
git -C "$production_tool" add .
expect_rejected production-tool "$production_tool"
grep -F 'tools/check/main.go:3: absolute Windows user-home path' "$tmp/production-tool.out" >/dev/null

production_identity=$(new_fixture production-identity)
mkdir -p "$production_identity/internal/core"
cat >"$production_identity/internal/core/config.go" <<'EOF'
package core

const releaseActor = "maintainer@example.com"
EOF
git -C "$production_identity" add .
expect_rejected production-identity "$production_identity"
grep -F 'personal identity or key material leaked outside isolated tests' \
  "$tmp/production-identity.out" >/dev/null

fixture_only=$(new_fixture fixture-only)
mkdir -p "$fixture_only/internal/core" "$fixture_only/scripts/tests/check" "$fixture_only/testdata"
cat >"$fixture_only/internal/core/config_test.go" <<'EOF'
package core

const fixturePath = "/Users/example/.config/product/config.toml"
EOF
printf '%s\n' '/home/example/product/config.toml' >"$fixture_only/scripts/tests/check/path.txt"
printf '%s\n' '192.168.10.20' >"$fixture_only/testdata/private-endpoint.txt"
git -C "$fixture_only" add .
"$checker" "$fixture_only" >/dev/null

fixed_python=$(new_fixture fixed-python)
mkdir -p "$fixed_python/scripts/checks"
cat >"$fixed_python/scripts/checks/run.sh" <<'EOF'
#!/bin/sh
exec /opt/homebrew/bin/python3.12 task.py
EOF
git -C "$fixed_python" add .
expect_rejected fixed-python "$fixed_python"
grep -F 'fixed Python interpreter path' "$tmp/fixed-python.out" >/dev/null

portable=$(new_fixture portable)
mkdir -p "$portable/internal/core"
cat >"$portable/internal/core/config.go" <<'EOF'
package core

const configPath = "config.toml"
EOF
git -C "$portable" add .
"$checker" "$portable" >/dev/null

printf '%s\n' 'portability regression fixtures: OK'
