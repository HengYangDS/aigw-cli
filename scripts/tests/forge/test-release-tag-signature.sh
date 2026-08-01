#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=core.hooksPath
export GIT_CONFIG_VALUE_0=/dev/null

cd "$tmp"
git init -q
git config user.name 'AIGW signature test'
git config user.email 'aigw-signature-test@example.invalid'
mkdir -p "$tmp/.config/release"
key="$tmp/signing"
ssh-keygen -q -t ed25519 -N '' -f "$key"
public=$(awk '{print $1" "$2}' "$key.pub")
printf 'fixture@example.invalid namespaces="git" %s\n' "$public" > "$tmp/allowed-signers"
printf 'fixture\n' > fixture
git add fixture
git commit -qm fixture
git tag -a v0.0.0-unsigned -m unsigned

if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed-signers" \
  sh "$root/scripts/checks/forge/check-release-tag-signature.sh" "$tmp" v0.0.0-unsigned >"$tmp/unsigned.out" 2>&1; then
  cat "$tmp/unsigned.out" >&2
  echo "unsigned annotated release tag was accepted" >&2
  exit 1
fi
grep -F 'not SSH signed' "$tmp/unsigned.out" >/dev/null || {
  cat "$tmp/unsigned.out" >&2
  echo "unsigned tag rejection did not identify the missing SSH signature" >&2
  exit 1
}

git -c user.name='Fixture signer' -c user.email='fixture@example.invalid' \
  -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
  -c user.signingkey="$key" tag -s -a v0.0.0-valid -m signed
AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed-signers" \
  sh "$root/scripts/checks/forge/check-release-tag-signature.sh" "$tmp" v0.0.0-valid gitlab >"$tmp/signed.out" 2>&1
signed=v0.0.0-valid

git update-ref refs/tags/github/v0.0.0-unsigned refs/tags/v0.0.0-unsigned
if AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed-signers" \
  sh "$root/scripts/checks/forge/check-release-tag-signature.sh" "$tmp" github/v0.0.0-unsigned github >"$tmp/github-qualified.out" 2>&1; then
  cat "$tmp/github-qualified.out" >&2
  echo "unsigned qualified GitHub tag was accepted" >&2
  exit 1
fi
grep -F 'not SSH signed' "$tmp/github-qualified.out" >/dev/null || {
  cat "$tmp/github-qualified.out" >&2
  echo "qualified GitHub tag was rejected before signature validation" >&2
  exit 1
}

if AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed-signers" \
  sh "$root/scripts/checks/forge/check-release-tag-signature.sh" "$tmp" github/v0.0.0-unsigned gitlab >"$tmp/github-qualified-provider.out" 2>&1; then
  cat "$tmp/github-qualified-provider.out" >&2
  echo "qualified GitHub tag was accepted under the GitLab provider" >&2
  exit 1
fi
grep -F 'qualified GitHub tag requires github provider' "$tmp/github-qualified-provider.out" >/dev/null || {
  cat "$tmp/github-qualified-provider.out" >&2
  echo "qualified GitHub tag provider rejection was not explicit" >&2
  exit 1
}

echo "release tag signature gate: OK ($signed; qualified namespace rejected unsigned reference)"
