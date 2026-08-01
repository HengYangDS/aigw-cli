#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
out=${1:?usage: test-linux-native-install.sh <dist-dir> <version>}
version=${2:?usage: test-linux-native-install.sh <dist-dir> <version>}
deb_image=${AIGW_LINUX_DEB_ACCEPTANCE_IMAGE:-ghcr.io/catthehacker/ubuntu:act-latest}
rpm_image=${AIGW_LINUX_RPM_ACCEPTANCE_IMAGE:-public.ecr.aws/docker/library/mysql:8.0}
shared_tmp_root=${AIGW_DOCKER_SHARED_TMPDIR:-"$HOME/.cache/aigw/container-artifacts"}
pull_timeout=${AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS:-120}
lock_timeout=${AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS:-180}
active_supervisor_pid=''
active_lock_dir=''

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required for Linux native-install acceptance: $1" >&2
    exit 2
  }
}

require_timeout_range() {
  name=$1
  value=$2
  if [ "$value" -ge 1 ] 2>/dev/null && [ "$value" -le 300 ] 2>/dev/null; then
    return 0
  fi
  echo "$name must be an integer from 1 through 300" >&2
  exit 2
}

require docker
require python3
require_timeout_range AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS "$pull_timeout"
require_timeout_range AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS "$lock_timeout"
[ -d "$out" ] || { echo "artifact directory does not exist: $out" >&2; exit 2; }
sh "$root/scripts/checks/release/check-release-artifacts.sh" "$out" "$version" >/dev/null

run_with_timeout() {
  seconds=$1
  shift
  python3 "$root/scripts/tests/install/run-with-timeout.py" "$seconds" "$@" &
  active_supervisor_pid=$!
  if wait "$active_supervisor_pid"; then
    supervisor_status=0
  else
    supervisor_status=$?
  fi
  active_supervisor_pid=''
  return "$supervisor_status"
}

image_lock_name() {
  printf '%s' "$1-$2" | tr '/:@' '___'
}

with_image_lock() {
  image=$1
  platform=$2
  shift 2
  lock_dir="$shared_tmp_root/.image-lock-$(image_lock_name "$image" "$platform")"
  elapsed=0
  while ! mkdir "$lock_dir" 2>/dev/null; do
    if [ "$elapsed" -ge "$lock_timeout" ]; then
      echo "timed out waiting for Linux acceptance image lock: $image ($platform)" >&2
      return 1
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  active_lock_dir=$lock_dir
  if "$@"; then
    lock_status=0
  else
    lock_status=$?
  fi
  rmdir "$active_lock_dir" 2>/dev/null || true
  active_lock_dir=''
  return "$lock_status"
}

ensure_image_platform() {
  image=$1
  platform=$2
  if docker image inspect --platform "$platform" --format '{{.Os}}/{{.Architecture}}' "$image" 2>/dev/null | grep -Fx "$platform" >/dev/null; then
    return 0
  fi
  mkdir -p "$shared_tmp_root"
  with_image_lock "$image" "$platform" ensure_image_platform_locked "$image" "$platform"
}

ensure_image_platform_locked() {
  image=$1
  platform=$2
  if docker image inspect --platform "$platform" --format '{{.Os}}/{{.Architecture}}' "$image" 2>/dev/null | grep -Fx "$platform" >/dev/null; then
    return 0
  fi
  printf 'preparing Linux acceptance image: %s (%s)\n' "$image" "$platform" >&2
  if run_with_timeout "$pull_timeout" docker pull --platform "$platform" "$image" >/dev/null; then
    :
  else
    status=$?
    if [ "$status" -eq 124 ]; then
      echo "timed out after ${pull_timeout}s while preparing Linux acceptance image: $image ($platform)" >&2
    else
      echo "failed to prepare Linux acceptance image: $image ($platform)" >&2
    fi
    return "$status"
  fi
  docker image inspect --platform "$platform" --format '{{.Os}}/{{.Architecture}}' "$image" 2>/dev/null | grep -Fx "$platform" >/dev/null || {
    echo "acceptance image $image is not available locally for $platform after pull" >&2
    exit 2
  }
}

mkdir -p "$shared_tmp_root"
staged=$(mktemp -d "$shared_tmp_root/aigw-linux-native-install.XXXXXX")
cleanup() {
  if [ -n "$active_lock_dir" ]; then
    rmdir "$active_lock_dir" 2>/dev/null || true
    active_lock_dir=''
  fi
  rm -rf "$staged"
}
forward_signal() {
  signal_name=$1
  signal_status=$2
  trap '' HUP INT TERM
  if [ -n "$active_supervisor_pid" ]; then
    kill "-$signal_name" "$active_supervisor_pid" 2>/dev/null || true
    wait "$active_supervisor_pid" 2>/dev/null || true
    active_supervisor_pid=''
  fi
  cleanup
  exit "$signal_status"
}
trap cleanup EXIT
trap 'forward_signal HUP 129' HUP
trap 'forward_signal INT 130' INT
trap 'forward_signal TERM 143' TERM
for arch in amd64 arm64; do
  cp "$out/aigw_${version}_linux_${arch}.deb" "$staged/"
  cp "$out/aigw_${version}_linux_${arch}.rpm" "$staged/"
done

for arch in amd64 arm64; do
  ensure_image_platform "$deb_image" "linux/$arch"
  docker run --pull never --platform "linux/$arch" --network none --rm --entrypoint /bin/sh -v "$staged:/artifacts:ro" \
    -e "AIGW_VERSION=$version" -e "AIGW_ARCH=$arch" -e "AIGW_PACKAGE_KIND=deb" "$deb_image" -exc '
      command -v dpkg >/dev/null || { echo "Debian acceptance image must provide dpkg" >&2; exit 2; }
      dpkg -i "/artifacts/aigw_${AIGW_VERSION}_linux_${AIGW_ARCH}.deb"
      test -x /usr/bin/aigw
      /usr/bin/aigw --version | grep -Fx "aigw version ${AIGW_VERSION}"
    '

  ensure_image_platform "$rpm_image" "linux/$arch"
  docker run --pull never --platform "linux/$arch" --network none --rm --entrypoint /bin/sh -v "$staged:/artifacts:ro" \
    -e "AIGW_VERSION=$version" -e "AIGW_ARCH=$arch" -e "AIGW_PACKAGE_KIND=rpm" "$rpm_image" -exc '
      command -v rpm >/dev/null || { echo "RPM acceptance image must provide rpm" >&2; exit 2; }
      rpm -i --nosignature "/artifacts/aigw_${AIGW_VERSION}_linux_${AIGW_ARCH}.rpm"
      test -x /usr/bin/aigw
      /usr/bin/aigw --version | grep -Fx "aigw version ${AIGW_VERSION}"
    '
done

echo "Linux package install paths: OK (amd64 and arm64 Debian/RPM compatibility harnesses)"
