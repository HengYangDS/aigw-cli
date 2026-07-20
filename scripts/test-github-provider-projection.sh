#!/bin/sh
# Regression contract for the provider-specific GitHub history projection.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
script="$root/scripts/project-github-forge.sh"
help=$(sh "$script" --help 2>&1 || true)
case "$help" in
  *"Existing GitHub release tags must verify as GitHub provenance"*) ;;
  *) echo 'GitHub projection help omits its provenance-verification boundary' >&2; exit 1 ;;
esac
case "$help" in
  *immutable*) echo 'GitHub projection help claims host-enforced tag immutability' >&2; exit 1 ;;
esac
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-github-provider-projection.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

source="$tmp/source"
remote="$tmp/github.git"
projection="$tmp/bootstrap-projection"
home="$tmp/home"
global_config="$tmp/global.gitconfig"
key="$tmp/signing"
mock_ssh="$tmp/mock-ssh"
mkdir -p "$home" "$tmp/allowed"
: > "$global_config"
ssh-keygen -q -t ed25519 -N '' -f "$key"
public=$(awk '{print $1" "$2}' "$key.pub")
printf 'heng.yang.ds@hotmail.com namespaces="git" %s\n' "$public" > "$tmp/allowed/gitlab"
printf 'hengyang.2003@tsinghua.org.cn namespaces="git" %s\n' "$public" > "$tmp/allowed/github"

cat > "$mock_ssh" <<'EOF'
#!/bin/sh
case "$*" in
  *git-upload-pack*) exec git-upload-pack "${AIGW_TEST_GITHUB_REMOTE:?}" ;;
  *git-receive-pack*) exec git-receive-pack "${AIGW_TEST_GITHUB_REMOTE:?}" ;;
esac
exit 0
EOF
chmod +x "$mock_ssh"

export HOME="$home"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL="$global_config"
# The user-level rewrite deliberately turns the raw SSH remote into an
# unusable HTTPS URL. Production must retain the local SSH remote and suppress
# this global rewrite only for GitHub transport operations.
git config --file "$global_config" url."https://github.com.invalid/".insteadOf git@github.com:

git init -q --bare "$remote"
git init -q -b main "$source"
git -C "$source" config user.name 'Yang HENG'
git -C "$source" config user.email 'heng.yang.ds@hotmail.com'
git -C "$source" config user.useConfigOnly true
printf 'first\n' > "$source/README.md"
git -C "$source" add README.md
git -C "$source" commit -qm 'first canonical source commit'
git -C "$source" -c gpg.format=ssh -c user.signingkey="$key" tag -s -a v0.1.0 -m 'GitLab release identity'
canonical_tag=$(git -C "$source" rev-parse refs/tags/v0.1.0)

# Bootstrap an existing GitHub provider history and its provider-native tag.
git clone -q --no-local "file://$source" "$projection"
git -C "$projection" tag -d v0.1.0 >/dev/null
FILTER_BRANCH_SQUELCH_WARNING=1 git -C "$projection" filter-branch -f --env-filter '
  GIT_AUTHOR_NAME="HengYang"
  GIT_AUTHOR_EMAIL="hengyang.2003@tsinghua.org.cn"
  GIT_COMMITTER_NAME="HengYang"
  GIT_COMMITTER_EMAIL="hengyang.2003@tsinghua.org.cn"
' -- main >/dev/null 2>&1
git -C "$projection" for-each-ref --format='%(refname)' refs/original/ | while IFS= read -r ref; do
  git -C "$projection" update-ref -d "$ref"
done
git -C "$projection" -c user.name=HengYang -c user.email=hengyang.2003@tsinghua.org.cn \
  -c gpg.format=ssh -c user.signingkey="$key" tag -s -a v0.1.0 -m 'GitHub release identity'
git -C "$projection" remote set-url origin "file://$remote"
git -C "$projection" -c core.hooksPath=/dev/null push -q origin main refs/tags/v0.1.0
remote_tag_before=$(git -C "$remote" rev-parse refs/tags/v0.1.0)

# Advance the canonical source. The production projection must not rewrite its
# refs or its GitLab release tag while it rewrites only the isolated clone.
printf 'second\n' >> "$source/README.md"
git -C "$source" add README.md
git -C "$source" commit -qm 'second canonical source commit'
source_head_before=$(git -C "$source" rev-parse HEAD)
source_refs_before=$(git -C "$source" for-each-ref --format='%(refname) %(objectname)' | LC_ALL=C sort)
git -C "$source" remote add github git@github.com:test/aigw-cli.git

(
  cd "$source"
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
    AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
    AIGW_TEST_GITHUB_REMOTE="$remote" \
    GIT_SSH_COMMAND="$mock_ssh" \
    AIGW_GITHUB_REMOTE=github \
    sh "$script" --branch main
) >/dev/null

[ "$(git -C "$source" rev-parse HEAD)" = "$source_head_before" ] || {
  echo 'projection rewrote canonical HEAD' >&2
  exit 1
}
[ "$(git -C "$source" for-each-ref --format='%(refname) %(objectname)' | LC_ALL=C sort)" = "$source_refs_before" ] || {
  echo 'projection rewrote canonical refs' >&2
  exit 1
}
[ "$(git -C "$source" rev-parse refs/tags/v0.1.0)" = "$canonical_tag" ] || {
  echo 'projection rewrote the GitLab release tag' >&2
  exit 1
}
[ "$(git -C "$remote" rev-parse refs/tags/v0.1.0)" = "$remote_tag_before" ] || {
  echo 'projection rewrote the GitHub release tag' >&2
  exit 1
}
[ "$(git -C "$remote" rev-parse refs/heads/main^{tree})" = "$(git -C "$source" rev-parse HEAD^{tree})" ] || {
  echo 'projected GitHub main tree differs from canonical source' >&2
  exit 1
}
if git -C "$remote" log main --format='%ae%n%ce' | grep -Fv -x 'hengyang.2003@tsinghua.org.cn' | grep -q .; then
  echo 'GitHub projection retains a non-GitHub identity' >&2
  exit 1
fi

# The projection may be invoked from an isolated worktree or any other local
# branch while `--branch main` selects a different canonical ref.  The initial
# clone follows the caller's current branch, so the implementation must detach
# at the requested source commit before it clears its temporary branch
# namespace; otherwise a later checkout sees every source file as staged.
git -C "$source" checkout -qb work/projection-caller-context main
printf 'caller-only\n' > "$source/CALLER_CONTEXT.txt"
git -C "$source" add CALLER_CONTEXT.txt
git -C "$source" commit -qm 'caller-only projection context'

(
  cd "$source"
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
    AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
    AIGW_TEST_GITHUB_REMOTE="$remote" \
    GIT_SSH_COMMAND="$mock_ssh" \
    AIGW_GITHUB_REMOTE=github \
    sh "$script" --branch main
) >/dev/null

[ "$(git -C "$remote" rev-parse refs/heads/main^{tree})" = "$(git -C "$source" rev-parse refs/heads/main^{tree})" ] || {
  echo 'projection from a non-canonical caller branch changed GitHub source content' >&2
  exit 1
}
if git -C "$remote" ls-tree -r --name-only main | grep -Fxq CALLER_CONTEXT.txt; then
  echo 'projection copied a caller-only worktree file into GitHub main' >&2
  exit 1
fi

# A GitHub-native provenance tag may have no same-named canonical GitLab tag,
# while still identifying a source tree represented by the selected canonical
# branch. It is nevertheless GitHub provenance and must be verified before the
# branch projection is updated.  An untrusted tag exercises the negative path:
# the old implementation checked only overlapping canonical tag names and
# would have accepted this remote state.
rogue_key="$tmp/rogue-signing"
ssh-keygen -q -t ed25519 -N '' -f "$rogue_key"
git -C "$remote" \
  -c user.name='Untrusted fixture signer' \
  -c user.email='untrusted@example.invalid' \
  -c gpg.format=ssh \
  -c user.signingkey="$rogue_key" \
  tag -s -a v0.2.0 main -m 'untrusted GitHub-native provenance identity'
source_main_before=$(git -C "$source" rev-parse refs/heads/main)
remote_main_before=$(git -C "$remote" rev-parse refs/heads/main)
if (
  cd "$source"
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
    AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
    AIGW_TEST_GITHUB_REMOTE="$remote" \
    GIT_SSH_COMMAND="$mock_ssh" \
    AIGW_GITHUB_REMOTE=github \
    sh "$script" --branch main
) >"$tmp/untrusted-github-only-tag.out" 2>&1; then
  cat "$tmp/untrusted-github-only-tag.out" >&2
  echo 'projection accepted an untrusted GitHub-native provenance tag' >&2
  exit 1
fi
grep -F 'GitHub provenance tag does not verify under its permitted trust anchors: v0.2.0' \
  "$tmp/untrusted-github-only-tag.out" >/dev/null || {
  cat "$tmp/untrusted-github-only-tag.out" >&2
  echo 'projection did not identify the untrusted GitHub-native provenance tag' >&2
  exit 1
}
[ "$(git -C "$source" rev-parse refs/heads/main)" = "$source_main_before" ] || {
  echo 'rejected GitHub-native provenance changed canonical main' >&2
  exit 1
}
[ "$(git -C "$remote" rev-parse refs/heads/main)" = "$remote_main_before" ] || {
  echo 'rejected GitHub-native provenance changed the GitHub main fixture' >&2
  exit 1
}
git -C "$remote" tag -d v0.2.0 >/dev/null

# A lightweight tag cannot carry the required provider signature and must be
# rejected explicitly rather than being treated as an ordinary object lookup.
git -C "$remote" tag v0.2.1 main
if (
  cd "$source"
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
    AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
    AIGW_TEST_GITHUB_REMOTE="$remote" \
    GIT_SSH_COMMAND="$mock_ssh" \
    AIGW_GITHUB_REMOTE=github \
    sh "$script" --branch main
) >"$tmp/lightweight-github-only-tag.out" 2>&1; then
  cat "$tmp/lightweight-github-only-tag.out" >&2
  echo 'projection accepted a lightweight GitHub-native provenance tag' >&2
  exit 1
fi
grep -F 'GitHub release tag must be annotated: v0.2.1' \
  "$tmp/lightweight-github-only-tag.out" >/dev/null || {
  cat "$tmp/lightweight-github-only-tag.out" >&2
  echo 'projection did not identify the lightweight GitHub-native provenance tag' >&2
  exit 1
}

echo 'GitHub provider projection isolation contract: OK'
