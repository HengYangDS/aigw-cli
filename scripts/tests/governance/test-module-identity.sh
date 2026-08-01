#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
checker="$root/scripts/checks/governance/check-module-identity.sh"
go_version=$(awk '$1 == "go" { print $2; exit }' "$root/go.mod")
[ -n "$go_version" ] || { echo "go.mod has no Go version" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

fixture() {
  name=$1
  module=$2
  path="$tmp/$name"
  mkdir -p "$path/cmd/aigw" "$path/internal/core"
  printf 'module %s\n\ngo %s\n' "$module" "$go_version" > "$path/go.mod"
  printf 'package main\n\nimport _ "%s/internal/core"\n\nfunc main() {}\n' "$module" > "$path/cmd/aigw/main.go"
  printf 'package core\n' > "$path/internal/core/core.go"
  printf '%s\n' "$path"
}

clean=$(fixture clean aigw-cli)
"$checker" "$clean" >/dev/null

for spec in \
  'private:gitlab.example.local/group/aigw-cli' \
  'personal:github.com/example-user/aigw-cli' \
  'url:https://example.test/aigw-cli' \
  'filesystem:/opt/team/aigw-cli'
do
  name=${spec%%:*}
  module=${spec#*:}
  path=$(fixture "$name" "$module")
  if "$checker" "$path" >"$tmp/$name.out" 2>&1; then
    echo "module identity checker accepted $name coordinate: $module" >&2
    exit 1
  fi
done

public=$(fixture public aigw-cli)
mkdir -p "$public/client"
printf 'package client\n' > "$public/client/client.go"
if "$checker" "$public" >"$tmp/public.out" 2>&1; then
  echo 'module identity checker accepted a public package under a non-fetchable module identity' >&2
  exit 1
fi
grep -F 'public Go packages require an explicitly owned, resolvable module identity' "$tmp/public.out" >/dev/null

foreign_import=$(fixture foreign-import aigw-cli)
printf 'package main\n\nimport _ "gitlab.example.local/group/aigw-cli/internal/core"\n\nfunc main() {}\n' > "$foreign_import/cmd/aigw/main.go"
if "$checker" "$foreign_import" >"$tmp/foreign-import.out" 2>&1; then
  echo 'module identity checker accepted a Forge-qualified internal import' >&2
  exit 1
fi
grep -F 'internal imports must use the product build identity' "$tmp/foreign-import.out" >/dev/null

printf '%s\n' 'module identity fixtures: OK'
