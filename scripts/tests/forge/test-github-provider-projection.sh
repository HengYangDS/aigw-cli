#!/bin/sh
# Regression contract for the provider-specific GitHub history projection.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
script="$root/scripts/forge/lib/project-github-forge.sh"
help=$(sh "$script" --help 2>&1 || true)
case "$help" in
  *"Existing GitHub release tags must verify as GitHub provenance"*) ;;
  *) echo 'GitHub projection help omits its provenance-verification boundary' >&2; exit 1 ;;
esac
case "$help" in
  *immutable*) echo 'GitHub projection help claims host-enforced tag immutability' >&2; exit 1 ;;
esac
case "$help" in
  *reconcile-divergence*) echo 'GitHub projection still offers history-rewriting divergence recovery' >&2; exit 1 ;;
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
signing_program="$tmp/signing-program"
signing_marker="$tmp/signing-program.called"
gitlab_email='gitlab@example.invalid'
github_name='GitHub Fixture'
github_email='github@example.invalid'
mkdir -p "$home" "$tmp/allowed"
: > "$global_config"
ssh-keygen -q -t ed25519 -N '' -f "$key"
public=$(awk '{print $1" "$2}' "$key.pub")
printf '%s namespaces="git" %s\n' "$gitlab_email" "$public" > "$tmp/allowed/gitlab"
printf '%s namespaces="git" %s\n' "$github_email" "$public" > "$tmp/allowed/github"

cat > "$mock_ssh" <<'EOF'
#!/bin/sh
case "$*" in
  *git-upload-pack*) exec git-upload-pack "${AIGW_TEST_GITHUB_REMOTE:?}" ;;
  *git-receive-pack*) exec git-receive-pack "${AIGW_TEST_GITHUB_REMOTE:?}" ;;
esac
exit 0
EOF
chmod +x "$mock_ssh"

ssh_keygen=$(command -v ssh-keygen)
cat > "$signing_program" <<EOF
#!/bin/sh
: > "\${AIGW_TEST_SIGNING_PROGRAM_MARKER:?}"
exec "$ssh_keygen" "\$@"
EOF
chmod +x "$signing_program"
export AIGW_GITHUB_SIGNING_PROGRAM="$signing_program"
export AIGW_TEST_SIGNING_PROGRAM_MARKER="$signing_marker"

export HOME="$home"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL="$global_config"
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=core.hooksPath
export GIT_CONFIG_VALUE_0=/dev/null
# The user-level rewrite deliberately turns the raw SSH remote into an
# unusable HTTPS URL. Production must retain the local SSH remote and suppress
# this global rewrite only for GitHub transport operations.
git config --file "$global_config" url."https://github.com.invalid/".insteadOf git@github.com:

git init -q --bare "$remote"
git init -q -b main "$source"
git -C "$source" config user.name 'GitLab Fixture'
git -C "$source" config user.email "$gitlab_email"
git -C "$source" config user.useConfigOnly true
git -C "$source" config gpg.format ssh
git -C "$source" config user.signingkey "$key"
git -C "$source" config commit.gpgsign true
printf 'first\n' > "$source/README.md"
git -C "$source" add README.md
git -C "$source" commit -qm 'first canonical source commit'
canonical_floor=$(git -C "$source" rev-parse HEAD)
git -C "$source" -c gpg.format=ssh -c user.signingkey="$key" tag -s -a v0.1.0 -m 'GitLab release identity'
canonical_tag=$(git -C "$source" rev-parse refs/tags/v0.1.0)
git -C "$source" commit --allow-empty -qm 'signed duplicate-tree canonical commit'

# Keep one signed branch forked before the initial GitHub synchronization. A
# later merge must map its old parent by identity-neutral commit fingerprint.
git -C "$source" checkout -qb work/old-parent "$canonical_floor"
printf 'old-parent\n' > "$source/OLD_PARENT.txt"
git -C "$source" add OLD_PARENT.txt
git -C "$source" commit -qm 'signed old-parent commit'
git -C "$source" checkout -q main

# Bootstrap an existing GitHub provider history through the same raw-object
# replay owner used by production. Tests must not maintain a second history
# rewriting algorithm or weaken the complete-history trust boundary.
go run "$root/tools/historyreplay" \
  --source "$source" \
  --revision main \
  --output "$projection" \
  --ref refs/heads/main \
  --actor-name "$github_name" \
  --actor-email "$github_email" \
  --signing-key "$key" \
  --allowed-signers "$tmp/allowed/github" >/dev/null
git -C "$projection" -c user.name="$github_name" -c user.email="$github_email" \
  -c gpg.format=ssh -c user.signingkey="$key" tag -s -a v0.1.0 -m 'GitHub release identity'
git -C "$projection" remote set-url origin "file://$remote"
git -C "$projection" -c core.hooksPath=/dev/null push -q origin refs/heads/main refs/tags/v0.1.0
git -C "$remote" symbolic-ref HEAD refs/heads/main
remote_tag_before=$(git -C "$remote" rev-parse refs/tags/v0.1.0)
remote_main_before=$(git -C "$remote" rev-parse refs/heads/main)
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
    AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
    AIGW_GITHUB_AUTHOR_NAME="$github_name" \
    AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
    AIGW_GITHUB_SIGNING_KEY="$key" \
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
remote_main_after=$(git -C "$remote" rev-parse refs/heads/main)
[ -f "$signing_marker" ] || {
  echo 'GitHub projection ignored its configured signing program' >&2
  exit 1
}
git -C "$remote" merge-base --is-ancestor "$remote_main_before" "$remote_main_after" || {
  echo 'GitHub projection rewrote the pre-policy branch tip' >&2
  exit 1
}
[ "$(git -C "$remote" rev-list --count "$remote_main_before..$remote_main_after")" -eq 1 ] || {
  echo 'GitHub projection did not append exactly one new source commit' >&2
  exit 1
}
git -C "$remote" \
  -c gpg.format=ssh \
  -c gpg.ssh.program=ssh-keygen \
  -c gpg.ssh.allowedSignersFile="$tmp/allowed/github" \
  verify-commit "$remote_main_after" >/dev/null 2>&1 || {
  echo 'GitHub projection did not sign its appended commit' >&2
  exit 1
}
if git -C "$remote" log main --format='%ae%n%ce' | grep -Fv -x "$github_email" | grep -q .; then
  echo 'GitHub projection retains a non-GitHub identity' >&2
  exit 1
fi

# Provider projection preserves canonical merge topology while appending only
# signed GitHub commits to the existing remote tip.
git -C "$source" checkout -qb work/provider-merge main
printf 'feature\n' > "$source/FEATURE.txt"
git -C "$source" add FEATURE.txt
git -C "$source" commit -qm 'signed feature commit'
git -C "$source" checkout -q main
printf 'main\n' > "$source/MAIN.txt"
git -C "$source" add MAIN.txt
git -C "$source" commit -qm 'signed main commit'
git -C "$source" merge -q --no-ff -S -m 'signed canonical merge' work/provider-merge
git -C "$source" merge -q --no-ff -S -m 'signed old-parent merge' work/old-parent
(
  cd "$source"
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
    AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
    AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
    AIGW_GITHUB_AUTHOR_NAME="$github_name" \
    AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
    AIGW_GITHUB_SIGNING_KEY="$key" \
    AIGW_TEST_GITHUB_REMOTE="$remote" \
    GIT_SSH_COMMAND="$mock_ssh" \
    AIGW_GITHUB_REMOTE=github \
    sh "$script" --branch main
) >/dev/null
remote_merge=$(git -C "$remote" rev-parse refs/heads/main)
[ "$(git -C "$remote" show -s --format='%P' "$remote_merge" | wc -w | tr -d ' ')" -eq 2 ] || {
  echo 'GitHub projection flattened a canonical merge commit' >&2
  exit 1
}
[ "$(git -C "$remote" rev-parse "$remote_merge^{tree}")" = "$(git -C "$source" rev-parse main^{tree})" ] || {
  echo 'GitHub merge projection tree differs from canonical source' >&2
  exit 1
}
for commit in $(git -C "$remote" rev-list "$remote_main_after..$remote_merge"); do
  git -C "$remote" \
    -c gpg.format=ssh \
    -c gpg.ssh.program=ssh-keygen \
    -c gpg.ssh.allowedSignersFile="$tmp/allowed/github" \
    verify-commit "$commit" >/dev/null 2>&1 || {
    echo "GitHub merge projection commit is unsigned: $commit" >&2
    exit 1
  }
done
remote_main_after=$remote_merge

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
    AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
    AIGW_GITHUB_AUTHOR_NAME="$github_name" \
    AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
    AIGW_GITHUB_SIGNING_KEY="$key" \
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
    AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
    AIGW_GITHUB_AUTHOR_NAME="$github_name" \
    AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
    AIGW_GITHUB_SIGNING_KEY="$key" \
    AIGW_TEST_GITHUB_REMOTE="$remote" \
    GIT_SSH_COMMAND="$mock_ssh" \
    AIGW_GITHUB_REMOTE=github \
    sh "$script" --branch main
) >"$tmp/untrusted-github-only-tag.out" 2>&1; then
  cat "$tmp/untrusted-github-only-tag.out" >&2
  echo 'projection accepted an untrusted GitHub-native provenance tag' >&2
  exit 1
fi
grep -F 'GitHub provenance tag does not verify: v0.2.0' \
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
    AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
    AIGW_GITHUB_AUTHOR_NAME="$github_name" \
    AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
    AIGW_GITHUB_SIGNING_KEY="$key" \
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
git -C "$remote" tag -d v0.2.1 >/dev/null

# A remote branch that contains an independently merged GitHub-only change is
# a divergence even when an operator later admits equivalent canonical work.
# Forward-only identity governance must refuse it without a rewrite escape.
git clone -q --no-local "file://$remote" "$tmp/divergent-github"
git -C "$tmp/divergent-github" config user.name "$github_name"
git -C "$tmp/divergent-github" config user.email "$github_email"
git -C "$tmp/divergent-github" config gpg.format ssh
git -C "$tmp/divergent-github" config user.signingkey "$key"
git -C "$tmp/divergent-github" config commit.gpgsign true
printf 'GitHub-only historical merge\n' > "$tmp/divergent-github/GITHUB_ONLY.txt"
git -C "$tmp/divergent-github" add GITHUB_ONLY.txt
git -C "$tmp/divergent-github" commit -qm 'historical GitHub-only merge'
git -C "$tmp/divergent-github" -c core.hooksPath=/dev/null push -q origin HEAD:main
remote_divergent_tip=$(git -C "$remote" rev-parse refs/heads/main)

git -C "$source" checkout -q main
printf 'canonical reconciliation\n' > "$source/CANONICAL_RECONCILIATION.txt"
git -C "$source" add CANONICAL_RECONCILIATION.txt
git -C "$source" commit -qm 'canonical reconciliation source'
source_main_before=$(git -C "$source" rev-parse refs/heads/main)

if (
  cd "$source"
  AIGW_GITHUB_ALLOWED_SIGNERS="$tmp/allowed/github" \
    AIGW_GITLAB_ALLOWED_SIGNERS="$tmp/allowed/gitlab" \
    AIGW_GITLAB_AUTHOR_EMAIL="$gitlab_email" \
    AIGW_GITHUB_AUTHOR_NAME="$github_name" \
    AIGW_GITHUB_AUTHOR_EMAIL="$github_email" \
    AIGW_GITHUB_SIGNING_KEY="$key" \
    AIGW_TEST_GITHUB_REMOTE="$remote" \
    GIT_SSH_COMMAND="$mock_ssh" \
    AIGW_GITHUB_REMOTE=github \
    sh "$script" --branch main
) >"$tmp/divergent-github.out" 2>&1; then
  cat "$tmp/divergent-github.out" >&2
  echo 'projection accepted a divergent GitHub main without explicit recovery' >&2
  exit 1
fi
grep -F 'GitHub branch diverges from the complete canonical identity projection; resolve manually' \
  "$tmp/divergent-github.out" >/dev/null || {
  cat "$tmp/divergent-github.out" >&2
  echo 'projection did not identify a divergent GitHub main' >&2
  exit 1
}
[ "$(git -C "$source" rev-parse refs/heads/main)" = "$source_main_before" ] || {
  echo 'rejected GitHub divergence changed canonical main' >&2
  exit 1
}
[ "$(git -C "$remote" rev-parse refs/heads/main)" = "$remote_divergent_tip" ] || {
  echo 'rejected GitHub divergence changed the remote main' >&2
  exit 1
}

echo 'GitHub provider projection isolation contract: OK'
