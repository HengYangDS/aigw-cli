#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)

fixture_process_is_live() {
  pid_file=$1
  [ -f "$pid_file" ] || return 1
  fixture_pid=$(cat "$pid_file")
  case "$fixture_pid" in
    ''|*[!0-9]*) return 1 ;;
  esac
  fixture_state=$(ps -p "$fixture_pid" -o stat= 2>/dev/null | awk 'NR == 1 { print $1 }')
  case "$fixture_state" in
    ''|Z*) return 1 ;;
  esac
  fixture_command=$(ps -p "$fixture_pid" -o command= 2>/dev/null || true)
  case "$fixture_command" in
    *"$tmp/timeout-bin/docker"*) return 0 ;;
    *) return 1 ;;
  esac
}

cleanup_timeout_fixture_processes() {
  for pid_file in \
    "$tmp/timeout-grandchild.pid" \
    "$tmp/timeout-child.pid" \
    "$tmp/timeout-root.pid"
  do
    if fixture_process_is_live "$pid_file"; then
      kill -KILL "$(cat "$pid_file")" 2>/dev/null || true
    fi
  done
}

cleanup() {
  cleanup_timeout_fixture_processes
  rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
out="$tmp/artifacts"
shared="$tmp/shared"
bin="$tmp/bin"
capture="$tmp/mounts"
mkdir -p "$out" "$shared" "$bin"

version=0.1.0-test
required="
aigw_${version}_darwin_amd64.tar.gz
aigw_${version}_darwin_arm64.tar.gz
aigw_${version}_darwin_universal.pkg
aigw_${version}_linux_amd64.deb
aigw_${version}_linux_amd64.rpm
aigw_${version}_linux_arm64.deb
aigw_${version}_linux_arm64.rpm
aigw_${version}_linux_amd64.tar.gz
aigw_${version}_linux_arm64.tar.gz
aigw_${version}_windows_amd64.msi
aigw_${version}_windows_amd64.zip
aigw_${version}_windows_arm64.msi
aigw_${version}_windows_arm64.zip
aigw_${version}.spdx.json
"

: > "$out/checksums.txt"
for name in $required; do
  printf 'fixture:%s\n' "$name" > "$out/$name"
  if command -v sha256sum >/dev/null 2>&1; then
    digest=$(sha256sum "$out/$name" | awk '{print $1}')
  else
    digest=$(shasum -a 256 "$out/$name" | awk '{print $1}')
  fi
  printf '%s  %s\n' "$digest" "$name" >> "$out/checksums.txt"
done

cat > "$bin/docker" <<'SH'
#!/bin/sh
set -eu
if [ "$1" = image ] && [ "$2" = inspect ]; then
  platform=''
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --platform) platform=$2; shift 2 ;;
      *) shift ;;
    esac
  done
  printf '%s\n' "$platform"
  exit 0
fi
if [ "$1" = pull ]; then
  exit 0
fi
mount=''
platform=''
network=''
pull=''
arch=''
package=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    -v)
      mount=$2
      shift 2
      ;;
    --platform)
      platform=$2
      shift 2
      ;;
    --network)
      network=$2
      shift 2
      ;;
    --pull)
      pull=$2
      shift 2
      ;;
    -e)
      case "$2" in
        AIGW_ARCH=*) arch=${2#AIGW_ARCH=} ;;
        AIGW_PACKAGE_KIND=*) package=${2#AIGW_PACKAGE_KIND=} ;;
      esac
      shift 2
      ;;
    *) shift ;;
  esac
done
[ -n "$mount" ] || { echo "missing artifact mount" >&2; exit 1; }
[ -n "$platform" ] || { echo "missing target platform" >&2; exit 1; }
[ "$network" = none ] || { echo "Linux package acceptance must run without a container network" >&2; exit 1; }
[ "$pull" = never ] || { echo "Linux package acceptance must never pull during package execution" >&2; exit 1; }
[ -n "$arch" ] || { echo "missing package architecture" >&2; exit 1; }
[ "$package" = deb ] || [ "$package" = rpm ] || { echo "missing package kind" >&2; exit 1; }
[ "$platform" = "linux/$arch" ] || { echo "target platform does not match package architecture" >&2; exit 1; }
host=${mount%%:*}
case "$host" in
  "$AIGW_DOCKER_SHARED_TMPDIR"/*) ;;
  *) echo "artifact mount was not staged in shared directory: $host" >&2; exit 1 ;;
esac
test -f "$host/aigw_0.1.0-test_linux_${arch}.${package}"
printf '%s %s %s\n' "$platform" "$package" "$host" >> "$AIGW_TEST_DOCKER_MOUNTS"
SH
chmod 755 "$bin/docker"

expect_timeout_upper_bound_rejected() {
  label=$1
  pull_timeout=$2
  lock_timeout=$3
  expected=$4
  output="$tmp/$label.out"
  if PATH="$bin:/usr/bin:/bin" \
    AIGW_DOCKER_SHARED_TMPDIR="$shared" \
    AIGW_TEST_DOCKER_MOUNTS="$capture" \
    AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS="$pull_timeout" \
    AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS="$lock_timeout" \
    AIGW_LINUX_DEB_ACCEPTANCE_IMAGE="example.test/debian" \
    AIGW_LINUX_RPM_ACCEPTANCE_IMAGE="example.test/rpm" \
    sh "$root/scripts/test-linux-native-install.sh" "$out" "$version" >"$output" 2>&1
  then
    echo "Linux native-install harness accepted $label timeout above 300 seconds" >&2
    return 1
  else
    rc=$?
  fi
  [ "$rc" -eq 2 ] || {
    cat "$output" >&2
    echo "Linux native-install harness used unexpected exit $rc for $label timeout above 300 seconds" >&2
    return 1
  }
  grep -F "$expected" "$output" >/dev/null || {
    cat "$output" >&2
    echo "Linux native-install harness did not explain $label timeout upper bound" >&2
    return 1
  }
}

upper_bound_failures=0
expect_timeout_upper_bound_rejected \
  image-pull 301 180 \
  "AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS must be an integer from 1 through 300" ||
  upper_bound_failures=$((upper_bound_failures + 1))
expect_timeout_upper_bound_rejected \
  image-lock 120 301 \
  "AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS must be an integer from 1 through 300" ||
  upper_bound_failures=$((upper_bound_failures + 1))
[ "$upper_bound_failures" -eq 0 ] || exit 1

PATH="$bin:/usr/bin:/bin" \
  AIGW_DOCKER_SHARED_TMPDIR="$shared" \
  AIGW_TEST_DOCKER_MOUNTS="$capture" \
  AIGW_LINUX_DEB_ACCEPTANCE_IMAGE="example.test/debian" \
  AIGW_LINUX_RPM_ACCEPTANCE_IMAGE="example.test/rpm" \
  sh "$root/scripts/test-linux-native-install.sh" "$out" "$version" >/dev/null

[ "$(wc -l < "$capture" | tr -d ' ')" = 4 ] || {
  cat "$capture" >&2
  echo "Linux native-install harness did not perform both package runs for both architectures" >&2
  exit 1
}
expected="$tmp/expected-runs"
cat > "$expected" <<'EOF'
linux/amd64 deb
linux/amd64 rpm
linux/arm64 deb
linux/arm64 rpm
EOF
awk '{print $1, $2}' "$capture" | sort > "$tmp/actual-runs"
diff -u "$expected" "$tmp/actual-runs"
while IFS=' ' read -r _ _ path; do
  [ ! -e "$path" ] || { echo "Linux native-install staging residue remains: $path" >&2; exit 1; }
done < "$capture"

timeout_bin="$tmp/timeout-bin"
mkdir -p "$timeout_bin"
for utility in awk cat cp dirname find grep mkdir mktemp ps rm rmdir sh sleep tr wc; do
  utility_path=$(command -v "$utility")
  ln -s "$utility_path" "$timeout_bin/$utility"
done
if command -v sha256sum >/dev/null 2>&1; then
  ln -s "$(command -v sha256sum)" "$timeout_bin/sha256sum"
else
  ln -s "$(command -v shasum)" "$timeout_bin/shasum"
fi
cat > "$timeout_bin/docker" <<'SH'
#!/bin/sh
case "${1-}" in
  __aigw_timeout_child)
    printf '%s\n' "$$" > "$AIGW_TEST_TIMEOUT_CHILD_PID"
    trap '' TERM
    "$0" __aigw_timeout_grandchild &
    wait "$!"
    ;;
  __aigw_timeout_grandchild)
    printf '%s\n' "$$" > "$AIGW_TEST_TIMEOUT_GRANDCHILD_PID"
    trap '' TERM
    while :; do sleep 1; done
    ;;
esac
case "$1:$2" in
  image:inspect) exit 1 ;;
  pull:*)
    printf '%s\n' "$$" > "$AIGW_TEST_TIMEOUT_ROOT_PID"
    trap 'exit 0' TERM
    "$0" __aigw_timeout_child &
    wait "$!"
    ;;
esac
exit 1
SH
chmod 755 "$timeout_bin/docker"
if PATH="$timeout_bin" command -v timeout >/dev/null 2>&1; then
  echo "Linux native-install fallback fixture unexpectedly provides timeout" >&2
  exit 1
fi
timeout_root_pid="$tmp/timeout-root.pid"
timeout_child_pid="$tmp/timeout-child.pid"
timeout_grandchild_pid="$tmp/timeout-grandchild.pid"
PATH="$timeout_bin" \
  AIGW_DOCKER_SHARED_TMPDIR="$shared" \
  AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS=1 \
  AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS=1 \
  AIGW_LINUX_DEB_ACCEPTANCE_IMAGE="example.test/debian" \
  AIGW_LINUX_RPM_ACCEPTANCE_IMAGE="example.test/rpm" \
  AIGW_TEST_TIMEOUT_ROOT_PID="$timeout_root_pid" \
  AIGW_TEST_TIMEOUT_CHILD_PID="$timeout_child_pid" \
  AIGW_TEST_TIMEOUT_GRANDCHILD_PID="$timeout_grandchild_pid" \
  sh "$root/scripts/test-linux-native-install.sh" "$out" "$version" >"$tmp/timeout.out" 2>&1 &
native_install_pid=$!
elapsed=0
while kill -0 "$native_install_pid" 2>/dev/null && [ "$elapsed" -lt 5 ]; do
  sleep 1
  elapsed=$((elapsed + 1))
done
if kill -0 "$native_install_pid" 2>/dev/null; then
  cleanup_timeout_fixture_processes
  kill -KILL "$native_install_pid" 2>/dev/null || true
  wait "$native_install_pid" 2>/dev/null || true
  cat "$tmp/timeout.out" >&2
  echo "Linux native-install fallback did not bound a descendant-resistant image pull" >&2
  exit 1
fi
if wait "$native_install_pid"; then
  echo "Linux native-install harness accepted an unbounded image pull" >&2
  exit 1
else
  rc=$?
fi
[ "$rc" -eq 124 ] || {
  cat "$tmp/timeout.out" >&2
  echo "Linux native-install harness used unexpected exit $rc for image-pull timeout" >&2
  exit 1
}
grep -F "timed out after 1s while preparing Linux acceptance image" "$tmp/timeout.out" >/dev/null || {
  cat "$tmp/timeout.out" >&2
  echo "Linux native-install harness did not explain image-pull timeout" >&2
  exit 1
}
for pid_file in "$timeout_root_pid" "$timeout_child_pid" "$timeout_grandchild_pid"; do
  [ -f "$pid_file" ] || {
    cat "$tmp/timeout.out" >&2
    echo "Linux native-install timeout fixture did not record its complete process tree" >&2
    exit 1
  }
done
if fixture_process_is_live "$timeout_root_pid"; then
  cleanup_timeout_fixture_processes
  echo "Linux native-install timeout left its TERM-exiting root alive" >&2
  exit 1
fi
leaked_descendants=''
for pid_file in "$timeout_child_pid" "$timeout_grandchild_pid"; do
  if fixture_process_is_live "$pid_file"; then
    leaked_descendants="$leaked_descendants $(cat "$pid_file")"
  fi
done
if [ -n "$leaked_descendants" ]; then
  cleanup_timeout_fixture_processes
  sleep 1
  for pid_file in "$timeout_child_pid" "$timeout_grandchild_pid"; do
    if fixture_process_is_live "$pid_file"; then
      echo "Linux native-install timeout fixture cleanup left a descendant alive" >&2
      exit 1
    fi
  done
  echo "Linux native-install fallback returned 124 but leaked retained descendant(s):$leaked_descendants; fixture cleanup left none" >&2
  exit 1
fi

lock_bin="$tmp/lock-bin"
mkdir -p "$lock_bin"
cat > "$lock_bin/docker" <<'SH'
#!/bin/sh
case "$1:$2" in
  image:inspect) exit 1 ;;
  pull:*) exit 0 ;;
esac
exit 1
SH
chmod 755 "$lock_bin/docker"
lock="$shared/.image-lock-example.test_debian-linux_amd64"
mkdir "$lock"
if PATH="$lock_bin:/usr/bin:/bin" \
  AIGW_DOCKER_SHARED_TMPDIR="$shared" \
  AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS=1 \
  AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS=1 \
  AIGW_LINUX_DEB_ACCEPTANCE_IMAGE="example.test/debian" \
  AIGW_LINUX_RPM_ACCEPTANCE_IMAGE="example.test/rpm" \
  sh "$root/scripts/test-linux-native-install.sh" "$out" "$version" >"$tmp/lock.out" 2>&1; then
  echo "Linux native-install harness ignored an active image lock" >&2
  exit 1
fi
rmdir "$lock"
grep -F "timed out waiting for Linux acceptance image lock" "$tmp/lock.out" >/dev/null || {
  cat "$tmp/lock.out" >&2
  echo "Linux native-install harness did not explain image-lock timeout" >&2
  exit 1
}

echo "Linux native-install shared-staging contract: OK"
