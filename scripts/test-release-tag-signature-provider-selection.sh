#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
fixture="$tmp/root"

mkdir -p "$fixture/scripts" "$fixture/packaging/release"
cp "$root/scripts/test-release-tag-signature.sh" "$fixture/scripts/test-release-tag-signature.sh"
printf 'fixture gitlab signer\n' > "$fixture/packaging/release/gitlab-allowed-signers"
printf 'fixture github signer\n' > "$fixture/packaging/release/github-allowed-signers"

cat > "$fixture/scripts/check-release-tag-signature.sh" <<'SH'
#!/bin/sh
set -eu

tag=${2:?missing tag}
provider=${3:-gitlab}
case "$tag" in
  v0.0.0-unsigned)
    echo "release tag is not SSH signed: $tag" >&2
    exit 1
    ;;
  v0.0.0-github)
    if test "$provider" = github; then
      exit 0
    fi
    echo "fixture release tag is only trusted by github: $provider" >&2
    exit 1
    ;;
  github/v0.0.0-unsigned)
    if test "$provider" = github; then
      echo "release tag is not SSH signed: $tag" >&2
      exit 1
    fi
    echo "qualified GitHub tag requires github provider: $tag" >&2
    exit 2
    ;;
  github/v0.0.0-github)
    if test "$provider" = github; then
      exit 0
    fi
    echo "qualified GitHub tag requires github provider: $tag" >&2
    exit 2
    ;;
  *)
    echo "unexpected fixture tag: $tag" >&2
    exit 2
    ;;
esac
SH
chmod +x "$fixture/scripts/check-release-tag-signature.sh"

git init -q "$fixture"
git -C "$fixture" config user.name 'AIGW provider-selection fixture'
git -C "$fixture" config user.email 'aigw-provider-selection@example.invalid'
printf 'fixture\n' > "$fixture/fixture"
git -C "$fixture" add fixture
git -C "$fixture" commit -qm fixture
git -C "$fixture" tag -a v0.0.0-unsigned -m unsigned
git -C "$fixture" tag -a v0.0.0-github -m github
git -C "$fixture" update-ref refs/tags/github/v0.0.0-github refs/tags/v0.0.0-github

if ! sh "$fixture/scripts/test-release-tag-signature.sh" > "$tmp/result.out" 2>&1; then
  cat "$tmp/result.out" >&2
  echo "release-tag signature regression did not discover a GitHub-native fixture" >&2
  exit 1
fi

grep -F 'release tag signature gate: OK (v0.0.0-github; qualified namespace rejected unsigned reference)' "$tmp/result.out" >/dev/null || {
  cat "$tmp/result.out" >&2
  echo "release-tag signature regression selected the wrong fixture" >&2
  exit 1
}

echo 'release tag provider-selection regression: OK'
