#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-git-hooks.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

for hook in pre-commit pre-push reference-transaction; do
  test -x "$root/.githooks/$hook" || {
    echo "hook must be tracked and executable: $hook" >&2
    exit 1
  }
done

repo=$tmp/repository
git -c init.templateDir= init -q "$repo"
git -C "$repo" config user.name 'AIGW Hook Test'
git -C "$repo" config user.email 'aigw-hook@example.invalid'
printf 'staged\n' > "$repo/tracked"
git -C "$repo" add tracked
mkdir -p "$tmp/bin"

write_ethos() {
  verdict=$1
  state=$2
  code=$3
  cat > "$tmp/bin/ethos" <<EOF
#!/bin/sh
printf '%s\\n' '{"verdict":"$verdict","state":"$state","required_gaps":["scope"]}'
exit $code
EOF
  chmod +x "$tmp/bin/ethos"
}

assert_quiet_success() {
  label=$1
  shift
  write_ethos pass admitted 0
  PATH="$tmp/bin:$PATH" "$@" >"$tmp/stdout" 2>"$tmp/stderr"
  test ! -s "$tmp/stdout" || { echo "$label must be quiet on stdout" >&2; exit 1; }
  test ! -s "$tmp/stderr" || { echo "$label must be quiet on stderr" >&2; exit 1; }
}

assert_actionable_failure() {
  label=$1
  shift
  write_ethos block blocked 1
  if PATH="$tmp/bin:$PATH" "$@" >"$tmp/stdout" 2>"$tmp/stderr"; then
    echo "$label unexpectedly succeeded" >&2
    exit 1
  fi
  test ! -s "$tmp/stdout" || { echo "$label must reserve stdout" >&2; exit 1; }
  grep -Fx '{"verdict":"block","state":"blocked","required_gaps":["scope"]}' \
    "$tmp/stderr" >/dev/null || {
      echo "$label must preserve actionable JSON" >&2
      exit 1
    }
}

assert_quiet_success pre-commit "$root/.githooks/pre-commit"
assert_actionable_failure pre-commit "$root/.githooks/pre-commit"

head=$(git -C "$repo" write-tree)
write_ethos pass admitted 0
printf 'refs/heads/local %s refs/heads/dev %040d\n' "$head" 0 | \
  PATH="$tmp/bin:$PATH" "$root/.githooks/pre-push" origin unused \
    >"$tmp/stdout" 2>"$tmp/stderr"
test ! -s "$tmp/stdout" || { echo 'pre-push must be quiet on stdout' >&2; exit 1; }
test ! -s "$tmp/stderr" || { echo 'pre-push must be quiet on stderr' >&2; exit 1; }

write_ethos block blocked 1
if printf 'refs/heads/local %s refs/heads/dev %040d\n' "$head" 0 | \
    PATH="$tmp/bin:$PATH" "$root/.githooks/pre-push" origin unused \
      >"$tmp/stdout" 2>"$tmp/stderr"; then
  echo 'blocked pre-push unexpectedly succeeded' >&2
  exit 1
fi
grep -Fx '{"verdict":"block","state":"blocked","required_gaps":["scope"]}' \
  "$tmp/stderr" >/dev/null || {
    echo 'blocked pre-push must preserve actionable JSON' >&2
    exit 1
  }

write_ethos pass admitted 0
printf '%040d %040d refs/heads/dev\n' 0 1 | \
  PATH="$tmp/bin:$PATH" "$root/.githooks/reference-transaction" prepared \
    >"$tmp/stdout" 2>"$tmp/stderr"
test ! -s "$tmp/stdout" || { echo 'reference-transaction must be quiet on stdout' >&2; exit 1; }
test ! -s "$tmp/stderr" || { echo 'reference-transaction must be quiet on stderr' >&2; exit 1; }

write_ethos block blocked 1
if printf '%040d %040d refs/heads/dev\n' 0 1 | \
    PATH="$tmp/bin:$PATH" "$root/.githooks/reference-transaction" prepared \
      >"$tmp/stdout" 2>"$tmp/stderr"; then
  echo 'blocked reference-transaction unexpectedly succeeded' >&2
  exit 1
fi
grep -Fx '{"verdict":"block","state":"blocked","required_gaps":["scope"]}' \
  "$tmp/stderr" >/dev/null || {
    echo 'blocked reference-transaction must preserve actionable JSON' >&2
    exit 1
  }

write_ethos pass admitted 0
if printf 'refs/heads/local %s refs/heads/work/private %040d\n' "$head" 0 | \
    PATH="$tmp/bin:$PATH" "$root/.githooks/pre-push" origin unused \
      >"$tmp/stdout" 2>"$tmp/stderr"; then
  echo 'private work branch push unexpectedly succeeded' >&2
  exit 1
fi
grep -F 'outside the shared namespace' "$tmp/stderr" >/dev/null || {
  echo 'pre-push rejection must explain the shared namespace' >&2
  exit 1
}

printf 'git hook tests: OK\n'
