#!/bin/sh
set -eu

root=${1:-$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)}
cd "$root"

fail() {
  echo "module identity contract failed: $1" >&2
  exit 1
}

[ -f go.mod ] || fail "missing go.mod"
module=$(awk 'NR == 1 && $1 == "module" && NF == 2 { print $2; exit }' go.mod)
[ "$module" = aigw-cli ] || fail "go.mod must use the non-fetchable product build identity aigw-cli"

case "$module" in
  *.*/*|*:*|/*|*\\*) fail "module identity must not encode a Forge, organization, host, URL, or filesystem path" ;;
esac

# AIGW exposes a command and repository-private packages, not an importable Go
# library. A future public package requires a separately owned, resolvable
# organization module path; it may not silently turn this build identity into a
# personal or deployment coordinate.
public_packages=$(find . -type f -name '*.go' \
  -not -path './.git/*' \
  -not -path './vendor/*' \
  -not -path './internal/*' \
  -not -path './cmd/*' \
  -not -path './tools/*' \
  -print)
[ -z "$public_packages" ] || {
  printf '%s\n' "$public_packages" >&2
  fail "public Go packages require an explicitly owned, resolvable module identity"
}

if find . -type f -name '*.go' -not -path './.git/*' -not -path './vendor/*' -exec grep -nHE '^[[:space:]]*(import[[:space:]]+)?([[:alnum:]_]+[[:space:]]+)?"(https?://|ssh://|git@|[^"/[:space:]]+\.[^"/[:space:]]+/[^"[:space:]]+)/internal/' {} + 2>/dev/null; then
  fail "internal imports must use the product build identity, never a Forge or transport coordinate"
fi

printf '%s\n' 'module identity contract: OK'
