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
  mkdir -p "$repo/docs/decisions"
  printf '%s\n' '# Decision Records' >"$repo/docs/decisions/README.md"
  printf '%s\n' "$repo"
}

expect_rejected() {
  label=$1
  repo=$2
	if "$checker" "$repo" >"$tmp/$label.out" 2>&1; then
    echo "portability checker accepted $label production hard-coding" >&2
    exit 1
	fi
	if grep -Fq 'cannot find main module' "$tmp/$label.out"; then
		echo "portability checker failed before evaluating $label" >&2
		exit 1
	fi
}

production_go=$(new_fixture production-go)
mkdir -p "$production_go/internal/core"
cat >"$production_go/internal/core/config.go" <<'EOF'
// Package core owns the fixture configuration.
package core

const configPath = "/Users/example/.config/product/config.toml"
EOF
git -C "$production_go" add .
expect_rejected production-go "$production_go"
grep -F '"rule": "absolute_user_home"' "$tmp/production-go.out" >/dev/null
grep -F '"path": "internal/core/config.go"' "$tmp/production-go.out" >/dev/null

untracked_go=$(new_fixture untracked-go)
mkdir -p "$untracked_go/internal/core"
cat >"$untracked_go/internal/core/config.go" <<'EOF'
// Package core owns the fixture configuration.
package core

const configPath = "/Users/example/.config/product/config.toml"
EOF
"$checker" "$untracked_go" >/dev/null

missing_file=$(new_fixture missing-file)
mkdir -p "$missing_file/internal/core"
printf '%s\n' '// Package core owns the fixture configuration.' 'package core' >"$missing_file/internal/core/config.go"
git -C "$missing_file" add .
rm "$missing_file/internal/core/config.go"
"$checker" "$missing_file" >/dev/null

symlink_file=$(new_fixture symlink-file)
mkdir -p "$symlink_file/internal/core"
printf '%s\n' 'portable' >"$symlink_file/target.txt"
ln -s ../../target.txt "$symlink_file/internal/core/config.go"
git -C "$symlink_file" add .
expect_rejected symlink-file "$symlink_file"
grep -F '"rule": "go_parse_error"' "$tmp/symlink-file.out" >/dev/null

production_tool=$(new_fixture production-tool)
mkdir -p "$production_tool/tools/check"
cat >"$production_tool/tools/check/main.go" <<'EOF'
// Command check owns the fixture verification.
package main

const configPath = `C:\Users\example\product\config.toml`
EOF
git -C "$production_tool" add .
expect_rejected production-tool "$production_tool"
grep -F '"rule": "absolute_windows_user_home"' "$tmp/production-tool.out" >/dev/null

production_identity=$(new_fixture production-identity)
mkdir -p "$production_identity/internal/core"
cat >"$production_identity/internal/core/config.go" <<'EOF'
// Package core owns the fixture configuration.
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

portable=$(new_fixture portable)
mkdir -p "$portable/internal/core"
cat >"$portable/internal/core/config.go" <<'EOF'
// Package core owns the fixture configuration.
package core

const configPath = "config.toml"
EOF
git -C "$portable" add .
"$checker" "$portable" >/dev/null

printf '%s\n' 'portability regression fixtures: OK'
