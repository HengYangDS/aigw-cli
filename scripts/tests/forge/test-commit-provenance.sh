#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
checker="$root/scripts/checks/forge/check-commit-provenance.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-commit-provenance.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/home" "$tmp/allowed"
export HOME="$tmp/home"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=core.hooksPath
export GIT_CONFIG_VALUE_0=/dev/null

gitlab_key="$tmp/gitlab-signing"
github_key="$tmp/github-signing"
rogue_key="$tmp/rogue-signing"
gitlab_email=gitlab@example.invalid
github_email=github@example.invalid
ssh-keygen -q -t ed25519 -N '' -f "$gitlab_key"
ssh-keygen -q -t ed25519 -N '' -f "$github_key"
ssh-keygen -q -t ed25519 -N '' -f "$rogue_key"
printf '%s namespaces="git" %s\n' "$gitlab_email" "$(cut -d ' ' -f 1,2 "$gitlab_key.pub")" > "$tmp/allowed/gitlab"
printf '%s namespaces="git" %s\n' "$github_email" "$(cut -d ' ' -f 1,2 "$github_key.pub")" > "$tmp/allowed/github"

init_repository() {
  repository=$1
  email=$2
  key=$3
  git init -q -b main "$repository"
  git -C "$repository" config user.name 'Provenance Fixture'
  git -C "$repository" config user.email "$email"
  git -C "$repository" config user.useConfigOnly true
  git -C "$repository" config gpg.format ssh
  git -C "$repository" config user.signingkey "$key"
  git -C "$repository" config commit.gpgsign true
}

signed_empty_commit() {
  git -C "$1" commit --allow-empty -S -qm "$2"
}

check_gitlab() {
  AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
    AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
    sh "$checker" "$1" gitlab
}

check_github() {
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
    AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
    sh "$checker" "$1" github
}

expect_failure() {
  name=$1
  expected=$2
  shift 2
  if "$@" >"$tmp/$name.out" 2>&1; then
    echo "commit provenance accepted $name" >&2
    exit 1
  fi
  grep -F "$expected" "$tmp/$name.out" >/dev/null || {
    cat "$tmp/$name.out" >&2
    echo "commit provenance did not identify $name" >&2
    exit 1
  }
}

valid_gitlab="$tmp/valid-gitlab"
init_repository "$valid_gitlab" "$gitlab_email" "$gitlab_key"
signed_empty_commit "$valid_gitlab" root
signed_empty_commit "$valid_gitlab" descendant
check_gitlab "$valid_gitlab" | grep -F 'gitlab commit provenance: 2 verified commit(s)' >/dev/null

invalid_root="$tmp/invalid-root"
init_repository "$invalid_root" legacy@example.invalid "$gitlab_key"
git -C "$invalid_root" commit --allow-empty --no-gpg-sign -qm 'invalid root'
git -C "$invalid_root" config user.email "$gitlab_email"
signed_empty_commit "$invalid_root" 'signed suffix'
expect_failure 'an invalid root hidden by a signed suffix' 'must use gitlab@example.invalid' \
  check_gitlab "$invalid_root"

unsigned_root="$tmp/unsigned-root"
init_repository "$unsigned_root" "$gitlab_email" "$gitlab_key"
git -C "$unsigned_root" commit --allow-empty --no-gpg-sign -qm 'unsigned root'
signed_empty_commit "$unsigned_root" 'signed suffix'
expect_failure 'an unsigned root hidden by a signed suffix' 'does not have a trusted gitlab signature' \
  check_gitlab "$unsigned_root"

wrong_author="$tmp/wrong-author"
init_repository "$wrong_author" "$gitlab_email" "$gitlab_key"
signed_empty_commit "$wrong_author" root
GIT_AUTHOR_EMAIL=wrong@example.invalid git -C "$wrong_author" commit --allow-empty -S -qm 'wrong author'
expect_failure 'a wrong author' 'must use gitlab@example.invalid' check_gitlab "$wrong_author"

wrong_committer="$tmp/wrong-committer"
init_repository "$wrong_committer" "$gitlab_email" "$gitlab_key"
signed_empty_commit "$wrong_committer" root
GIT_COMMITTER_EMAIL=wrong@example.invalid git -C "$wrong_committer" commit --allow-empty -S -qm 'wrong committer'
expect_failure 'a wrong committer' 'must use gitlab@example.invalid' check_gitlab "$wrong_committer"

untrusted="$tmp/untrusted"
init_repository "$untrusted" "$github_email" "$rogue_key"
signed_empty_commit "$untrusted" 'untrusted GitHub root'
expect_failure 'an untrusted GitHub signer' 'does not have a trusted github signature' \
  check_github "$untrusted"

merge_parent="$tmp/merge-parent"
init_repository "$merge_parent" "$gitlab_email" "$gitlab_key"
printf 'root\n' > "$merge_parent/root"
git -C "$merge_parent" add root
git -C "$merge_parent" commit -S -qm root
git -C "$merge_parent" checkout -qb feature
git -C "$merge_parent" config user.signingkey "$rogue_key"
printf 'feature\n' > "$merge_parent/feature"
git -C "$merge_parent" add feature
git -C "$merge_parent" commit -S -qm 'untrusted merge parent'
rogue_parent=$(git -C "$merge_parent" rev-parse HEAD)
git -C "$merge_parent" checkout -q main
git -C "$merge_parent" config user.signingkey "$gitlab_key"
printf 'main\n' > "$merge_parent/main"
git -C "$merge_parent" add main
git -C "$merge_parent" commit -S -qm main
git -C "$merge_parent" merge -q --no-ff -S -m merge feature
expect_failure 'an untrusted merge parent' "$rogue_parent does not have a trusted gitlab signature" \
  check_gitlab "$merge_parent"

touch "$valid_gitlab/.mailmap"
expect_failure 'a mailmap identity overlay' '.mailmap is forbidden' check_gitlab "$valid_gitlab"
rm "$valid_gitlab/.mailmap"

expect_failure 'an implicit publication actor' \
  'author email is required through AIGW_GITLAB_AUTHOR_EMAIL' \
  env AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" sh "$checker" "$valid_gitlab" gitlab
expect_failure 'an empty trust input' \
  'trust input is required through AIGW_GITLAB_ALLOWED_SIGNERS' \
  env AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" sh "$checker" "$valid_gitlab" gitlab
expect_failure 'an unknown provider' 'provider must be gitlab or github' \
  sh "$checker" "$valid_gitlab" unknown
expect_failure 'a retired range argument' \
  'usage: check-commit-provenance.sh <repository> <gitlab|github>' \
  sh "$checker" "$valid_gitlab" gitlab HEAD~1

echo 'commit provenance tests: OK'
