#!/bin/sh
# Verify that release-tag namespaces retain one unambiguous owner per forge.
# A canonical local checkout keeps GitLab tags unscoped and fetched GitHub
# provenance below github/. Native forge checkouts keep their own tags
# unscoped. Legacy provider/ aliases are always rejected.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

fail() {
  echo "release tag namespace: $*" >&2
  exit 1
}

forge=${AIGW_TAG_NAMESPACE_FORGE:-}
if test -z "$forge"; then
  if test "${GITHUB_ACTIONS:-}" = true; then
    forge=github
  elif test -n "${GITLAB_CI:-}"; then
    forge=gitlab
  elif git for-each-ref --format='%(refname)' 'refs/tags/github/v[0-9]*' | grep -q .; then
    # A canonical checkout retains fetched GitHub provenance in a qualified
    # namespace.  Prefer that concrete local topology over remote URL guesses.
    forge=local
  else
    origin_url=$(git config --local --get remote.origin.url 2>/dev/null || true)
    case "$origin_url" in
      *github.com*)
        # A standalone GitHub clone must be verifiable outside Actions as well
        # as inside it. Its unscoped tags are GitHub provenance, not GitLab
        # aliases.
        forge=github
        ;;
      *) forge=local ;;
    esac
  fi
fi
case "$forge" in
  local|gitlab|github) ;;
  *) fail "invalid forge mode: $forge" ;;
esac

gitlab_allowed_signers=${AIGW_GITLAB_ALLOWED_SIGNERS:-}
github_allowed_signers=${AIGW_GITHUB_ALLOWED_SIGNERS:-}
github_legacy_allowed_signers=${AIGW_GITHUB_LEGACY_ALLOWED_SIGNERS:-}
github_legacy_tags=${AIGW_GITHUB_LEGACY_TAGS:-$root/.config/release/github-legacy-tags.txt}

case "$forge" in
  local)
    test -n "$gitlab_allowed_signers" || fail "GitLab trust input is required"
    test -n "$github_allowed_signers" || fail "GitHub trust input is required"
    ;;
  gitlab) test -n "$gitlab_allowed_signers" || fail "GitLab trust input is required" ;;
  github) test -n "$github_allowed_signers" || fail "GitHub trust input is required" ;;
esac

verify_tag() {
  provider=$1
  tag=$2
  case "$provider" in
    gitlab) allowed_signers=$gitlab_allowed_signers ;;
    github)
      if git -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
        -c gpg.ssh.allowedSignersFile="$github_allowed_signers" \
        verify-tag "$tag" >/dev/null 2>&1; then
        return
      fi
      tag_name=${tag#github/}
      if test -f "$github_legacy_tags" && grep -Fxq "$tag_name" "$github_legacy_tags" && \
        git -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
          -c gpg.ssh.allowedSignersFile="$github_legacy_allowed_signers" \
          verify-tag "$tag" >/dev/null 2>&1; then
        return
      fi
      fail "github tag does not verify: $tag"
      ;;
    *) fail "invalid tag provider: $provider" ;;
  esac
  git -c gpg.format=ssh -c gpg.ssh.program=ssh-keygen \
    -c gpg.ssh.allowedSignersFile="$allowed_signers" \
    verify-tag "$tag" >/dev/null 2>&1 || \
    fail "$provider tag does not verify: $tag"
}

git for-each-ref --format='%(refname:short)' refs/tags | while IFS= read -r tag; do
  case "$tag" in
    github/v[0-9]*.*.*)
      test "$forge" = local || fail "qualified GitHub provenance is only valid in a local canonical checkout: $tag"
      verify_tag github "$tag"
      ;;
    provider/*)
      fail "legacy provider alias remains: $tag"
      ;;
    v[0-9]*.*.*)
      case "$forge" in
        github) verify_tag github "$tag" ;;
        local|gitlab) verify_tag gitlab "$tag" ;;
      esac
      ;;
    *)
      fail "unexpected release tag namespace: $tag"
      ;;
  esac
done

printf 'release tag namespace: OK (%s)\n' "$forge"
