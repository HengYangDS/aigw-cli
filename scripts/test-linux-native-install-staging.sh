#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
real_python3=$(command -v python3)

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
    *"$tmp/"*) return 0 ;;
    *) return 1 ;;
  esac
}

cleanup_timeout_fixture_processes() {
  for pid_file in \
    "$tmp/supervisor-cancel-child.pid" \
    "$tmp/supervisor-cancel-root.pid" \
    "$tmp/supervisor-completed-child.pid" \
    "$tmp/supervisor-late-child.pid" \
    "$tmp/supervisor-late-root.pid" \
    "$tmp/supervisor-sentinel.pid" \
    "$tmp/supervisor-timeout.pid" \
    "$tmp/supervisor-python.pid" \
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

supervisor_bin="$tmp/supervisor-bin"
mkdir -p "$supervisor_bin"
for utility in awk cat cp dirname find grep mkdir mktemp rm rmdir sh sleep tr wc; do
  ln -s "$(command -v "$utility")" "$supervisor_bin/$utility"
done
if command -v sha256sum >/dev/null 2>&1; then
  ln -s "$(command -v sha256sum)" "$supervisor_bin/sha256sum"
else
  ln -s "$(command -v shasum)" "$supervisor_bin/shasum"
fi
cat > "$supervisor_bin/python3" <<'SH'
#!/bin/sh
printf '%s\n' "$$" > "$AIGW_TEST_SUPERVISOR_PID"
exec "$AIGW_TEST_REAL_PYTHON3" "$@"
SH
cat > "$supervisor_bin/timeout" <<'SH'
#!/bin/sh
printf '%s\n' "$$" > "$AIGW_TEST_FAKE_TIMEOUT_PID"
: > "$AIGW_TEST_FAKE_TIMEOUT_INVOKED"
trap '' TERM
while :; do sleep 1; done
SH
cat > "$supervisor_bin/docker" <<'SH'
#!/bin/sh
case "${1-}" in
  __aigw_sentinel)
    printf '%s\n' "$$" > "$AIGW_TEST_SENTINEL_PID"
    trap '' TERM
    trap 'exit 0' HUP
    while :; do sleep 1; done
    ;;
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
  __aigw_late_child)
    printf '%s\n' "$$" > "$AIGW_TEST_LATE_CHILD_PID"
    trap '' TERM
    while :; do sleep 1; done
    ;;
  __aigw_cancel_child)
    printf '%s\n' "$$" > "$AIGW_TEST_CANCEL_CHILD_PID"
    trap '' TERM
    while :; do sleep 1; done
    ;;
  __aigw_completed_child)
    printf '%s\n' "$$" > "$AIGW_TEST_COMPLETED_CHILD_PID"
    trap '' TERM
    while :; do sleep 1; done
    ;;
  __aigw_completed_leader)
    "$0" __aigw_completed_child &
    while [ ! -s "$AIGW_TEST_COMPLETED_CHILD_PID" ]; do :; done
    exit 0
    ;;
esac
case "$1:$2" in
  image:inspect) exit 1 ;;
  pull:*)
    case "$AIGW_TEST_DOCKER_MODE" in
      root-exit)
        printf '%s\n' "$$" > "$AIGW_TEST_TIMEOUT_ROOT_PID"
        trap 'exit 0' TERM
        "$0" __aigw_timeout_child &
        wait "$!"
        ;;
      late-fork)
        printf '%s\n' "$$" > "$AIGW_TEST_LATE_ROOT_PID"
        trap 'printf "reused:%s\n" "$(cat "$AIGW_TEST_SENTINEL_PID")" > "$AIGW_TEST_IDENTITY_SWITCH"; sleep 0.2; "$0" __aigw_late_child & while [ ! -s "$AIGW_TEST_LATE_CHILD_PID" ]; do :; done' TERM
        while :; do sleep 1; done
        ;;
      cancellation)
        printf '%s\n' "$$" > "$AIGW_TEST_CANCEL_ROOT_PID"
        trap '' TERM
        "$0" __aigw_cancel_child &
        while [ ! -s "$AIGW_TEST_CANCEL_CHILD_PID" ]; do :; done
        wait "$!"
        ;;
    esac
    ;;
esac
exit 1
SH
chmod 755 "$supervisor_bin/python3" "$supervisor_bin/timeout" "$supervisor_bin/docker"

timeout_root_pid="$tmp/timeout-root.pid"
timeout_child_pid="$tmp/timeout-child.pid"
timeout_grandchild_pid="$tmp/timeout-grandchild.pid"
supervisor_late_root_pid="$tmp/supervisor-late-root.pid"
supervisor_late_child_pid="$tmp/supervisor-late-child.pid"
supervisor_cancel_root_pid="$tmp/supervisor-cancel-root.pid"
supervisor_cancel_child_pid="$tmp/supervisor-cancel-child.pid"
supervisor_completed_child_pid="$tmp/supervisor-completed-child.pid"
supervisor_sentinel_pid="$tmp/supervisor-sentinel.pid"
supervisor_python_pid="$tmp/supervisor-python.pid"
supervisor_timeout_pid="$tmp/supervisor-timeout.pid"
supervisor_timeout_invoked="$tmp/supervisor-timeout-invoked"
supervisor_identity_switch="$tmp/supervisor-identity-switched"

if AIGW_TEST_COMPLETED_CHILD_PID="$supervisor_completed_child_pid" \
  "$real_python3" "$root/scripts/run-with-timeout.py" 5 \
  "$supervisor_bin/docker" __aigw_completed_leader
then
  :
else
  echo "Linux timeout supervisor changed a completed leader's exit status" >&2
  exit 1
fi
[ -s "$supervisor_completed_child_pid" ] || {
  echo "Linux timeout supervisor completion fixture did not create its descendant" >&2
  exit 1
}
if fixture_process_is_live "$supervisor_completed_child_pid"; then
  cleanup_timeout_fixture_processes
  sleep 1
  fixture_process_is_live "$supervisor_completed_child_pid" && {
    echo "Linux timeout supervisor fixture cleanup left a descendant running" >&2
    exit 1
  }
  echo "Linux timeout supervisor returned after its leader while an owned descendant was still running" >&2
  exit 1
fi

start_supervisor_case() {
  supervisor_label=$1
  supervisor_mode=$2
  supervisor_timeout=$3
  supervisor_output="$tmp/$supervisor_label.out"
  PATH="$supervisor_bin" \
    AIGW_DOCKER_SHARED_TMPDIR="$shared" \
    AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS="$supervisor_timeout" \
    AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS=1 \
    AIGW_LINUX_DEB_ACCEPTANCE_IMAGE="example.test/debian" \
    AIGW_LINUX_RPM_ACCEPTANCE_IMAGE="example.test/rpm" \
    AIGW_TEST_REAL_PYTHON3="$real_python3" \
    AIGW_TEST_SUPERVISOR_PID="$supervisor_python_pid" \
    AIGW_TEST_FAKE_TIMEOUT_PID="$supervisor_timeout_pid" \
    AIGW_TEST_FAKE_TIMEOUT_INVOKED="$supervisor_timeout_invoked" \
    AIGW_TEST_DOCKER_MODE="$supervisor_mode" \
    AIGW_TEST_TIMEOUT_ROOT_PID="$timeout_root_pid" \
    AIGW_TEST_TIMEOUT_CHILD_PID="$timeout_child_pid" \
    AIGW_TEST_TIMEOUT_GRANDCHILD_PID="$timeout_grandchild_pid" \
    AIGW_TEST_LATE_ROOT_PID="$supervisor_late_root_pid" \
    AIGW_TEST_LATE_CHILD_PID="$supervisor_late_child_pid" \
    AIGW_TEST_CANCEL_ROOT_PID="$supervisor_cancel_root_pid" \
    AIGW_TEST_CANCEL_CHILD_PID="$supervisor_cancel_child_pid" \
    AIGW_TEST_SENTINEL_PID="$supervisor_sentinel_pid" \
    AIGW_TEST_IDENTITY_SWITCH="$supervisor_identity_switch" \
    sh "$root/scripts/test-linux-native-install.sh" "$out" "$version" >"$supervisor_output" 2>&1 &
  supervisor_native_pid=$!
}

wait_supervisor_case() {
  supervisor_limit=$1
  supervisor_elapsed=0
  while kill -0 "$supervisor_native_pid" 2>/dev/null && [ "$supervisor_elapsed" -lt "$supervisor_limit" ]; do
    sleep 1
    supervisor_elapsed=$((supervisor_elapsed + 1))
  done
  if kill -0 "$supervisor_native_pid" 2>/dev/null; then
    cleanup_timeout_fixture_processes
    kill -KILL "$supervisor_native_pid" 2>/dev/null || true
    wait "$supervisor_native_pid" 2>/dev/null || true
    cat "$supervisor_output" >&2
    echo "Linux native-install safe supervisor exceeded its bounded fixture deadline" >&2
    exit 1
  fi
  if wait "$supervisor_native_pid"; then
    supervisor_rc=0
  else
    supervisor_rc=$?
  fi
}

start_supervisor_case root-exit root-exit 1
wait_supervisor_case 6
[ "$supervisor_rc" -eq 124 ] || {
  cat "$supervisor_output" >&2
  echo "Linux native-install root-exit fixture used unexpected exit $supervisor_rc" >&2
  exit 1
}
grep -F "timed out after 1s while preparing Linux acceptance image" "$supervisor_output" >/dev/null
for pid_file in "$timeout_root_pid" "$timeout_child_pid" "$timeout_grandchild_pid"; do
  [ -s "$pid_file" ] || { echo "Linux native-install root-exit fixture missed a process" >&2; exit 1; }
  ! fixture_process_is_live "$pid_file" || { echo "Linux native-install root-exit fixture leaked a process" >&2; exit 1; }
done
[ ! -e "$supervisor_timeout_invoked" ] || { echo "Linux native-install invoked an untrusted timeout executable" >&2; exit 1; }

AIGW_TEST_SENTINEL_PID="$supervisor_sentinel_pid" "$supervisor_bin/docker" __aigw_sentinel &
supervisor_sentinel_job=$!
while [ ! -s "$supervisor_sentinel_pid" ]; do sleep 1; done
identity_before="retained:$(cat "$supervisor_sentinel_pid")"
start_supervisor_case late-fork late-fork 1
wait_supervisor_case 6
[ "$supervisor_rc" -eq 124 ] || {
  cat "$supervisor_output" >&2
  echo "Linux native-install late-fork fixture used unexpected exit $supervisor_rc" >&2
  exit 1
}
[ -s "$supervisor_late_child_pid" ] && [ -f "$supervisor_identity_switch" ] || {
  echo "Linux native-install late-fork fixture did not create its adversarial descendant" >&2
  exit 1
}
identity_after=$(cat "$supervisor_identity_switch")
[ "$identity_before" != "$identity_after" ] || { echo "Linux native-install identity simulation did not change" >&2; exit 1; }
fixture_process_is_live "$supervisor_sentinel_pid" || { echo "Linux native-install targeted an unrelated reused identity" >&2; exit 1; }
! fixture_process_is_live "$supervisor_late_child_pid" || { echo "Linux native-install late-fork descendant escaped" >&2; exit 1; }
[ ! -e "$supervisor_timeout_invoked" ] || { echo "Linux native-install invoked an untrusted timeout executable" >&2; exit 1; }
kill -HUP "$(cat "$supervisor_sentinel_pid")"
wait "$supervisor_sentinel_job" 2>/dev/null || true

: > "$supervisor_python_pid"
start_supervisor_case cancellation cancellation 30
supervisor_elapsed=0
while { [ ! -s "$supervisor_python_pid" ] || [ ! -s "$supervisor_cancel_root_pid" ] || [ ! -s "$supervisor_cancel_child_pid" ]; } &&
  [ "$supervisor_elapsed" -lt 5 ]
do
  sleep 1
  supervisor_elapsed=$((supervisor_elapsed + 1))
done
for pid_file in "$supervisor_python_pid" "$supervisor_cancel_root_pid" "$supervisor_cancel_child_pid"; do
  [ -s "$pid_file" ] || { echo "Linux native-install cancellation fixture missed a process" >&2; exit 1; }
done
kill -TERM "$supervisor_native_pid"
wait_supervisor_case 5
[ "$supervisor_rc" -eq 143 ] || {
  cat "$supervisor_output" >&2
  echo "Linux native-install cancellation fixture used unexpected exit $supervisor_rc" >&2
  exit 1
}
for pid_file in "$supervisor_python_pid" "$supervisor_cancel_root_pid" "$supervisor_cancel_child_pid"; do
  ! fixture_process_is_live "$pid_file" || { echo "Linux native-install cancellation leaked an owned process" >&2; exit 1; }
done
[ ! -d "$shared/.image-lock-example.test_debian-linux_amd64" ] || {
  echo "Linux native-install cancellation leaked its image lock" >&2
  exit 1
}

missing_supervisor_bin="$tmp/missing-supervisor-bin"
mkdir -p "$missing_supervisor_bin"
ln -s "$(command -v dirname)" "$missing_supervisor_bin/dirname"
ln -s "$(command -v sh)" "$missing_supervisor_bin/sh"
cat > "$missing_supervisor_bin/docker" <<'SH'
#!/bin/sh
: > "$AIGW_TEST_MISSING_SUPERVISOR_DOCKER_LOG"
exit 1
SH
chmod 755 "$missing_supervisor_bin/docker"
missing_supervisor_log="$tmp/missing-supervisor-docker.log"
if PATH="$missing_supervisor_bin" AIGW_TEST_MISSING_SUPERVISOR_DOCKER_LOG="$missing_supervisor_log" \
  sh "$root/scripts/test-linux-native-install.sh" "$out" "$version" >"$tmp/missing-supervisor.out" 2>&1
then
  echo "Linux native-install ran without a safe supervisor" >&2
  exit 1
else
  missing_supervisor_rc=$?
fi
[ "$missing_supervisor_rc" -eq 2 ] && [ ! -e "$missing_supervisor_log" ] || {
  echo "Linux native-install launched Docker without a safe supervisor" >&2
  exit 1
}
grep -F "required for Linux native-install acceptance: python3" "$tmp/missing-supervisor.out" >/dev/null

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
if PATH="$lock_bin:$PATH" \
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

mkdir "$lock"
PATH="$lock_bin:$PATH" \
  AIGW_DOCKER_SHARED_TMPDIR="$shared" \
  AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS=1 \
  AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS=30 \
  AIGW_LINUX_DEB_ACCEPTANCE_IMAGE="example.test/debian" \
  AIGW_LINUX_RPM_ACCEPTANCE_IMAGE="example.test/rpm" \
  sh "$root/scripts/test-linux-native-install.sh" "$out" "$version" >"$tmp/lock-cancel.out" 2>&1 &
lock_cancel_pid=$!
sleep 1
kill -TERM "$lock_cancel_pid"
lock_cancel_elapsed=0
while kill -0 "$lock_cancel_pid" 2>/dev/null && [ "$lock_cancel_elapsed" -lt 5 ]; do
  sleep 1
  lock_cancel_elapsed=$((lock_cancel_elapsed + 1))
done
if kill -0 "$lock_cancel_pid" 2>/dev/null; then
  kill -KILL "$lock_cancel_pid" 2>/dev/null || true
  wait "$lock_cancel_pid" 2>/dev/null || true
  echo "Linux native-install did not cancel bounded lock waiting" >&2
  exit 1
fi
if wait "$lock_cancel_pid"; then
  echo "Linux native-install lock-wait cancellation unexpectedly succeeded" >&2
  exit 1
else
  lock_cancel_rc=$?
fi
[ "$lock_cancel_rc" -eq 143 ] || { echo "Linux native-install lock-wait cancellation used exit $lock_cancel_rc" >&2; exit 1; }
[ -d "$lock" ] || { echo "Linux native-install cancellation removed a foreign image lock" >&2; exit 1; }
rmdir "$lock"

echo "Linux native-install shared-staging contract: OK"
