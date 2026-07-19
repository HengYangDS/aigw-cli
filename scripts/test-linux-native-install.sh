#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
out=${1:?usage: test-linux-native-install.sh <dist-dir> <version>}
version=${2:?usage: test-linux-native-install.sh <dist-dir> <version>}
deb_image=${AIGW_LINUX_DEB_ACCEPTANCE_IMAGE:-ghcr.io/catthehacker/ubuntu:act-latest}
rpm_image=${AIGW_LINUX_RPM_ACCEPTANCE_IMAGE:-public.ecr.aws/docker/library/mysql:8.0}
shared_tmp_root=${AIGW_DOCKER_SHARED_TMPDIR:-"$HOME/.cache/aigw/container-artifacts"}
pull_timeout=${AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS:-120}
lock_timeout=${AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS:-180}

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
require_timeout_range AIGW_LINUX_IMAGE_PULL_TIMEOUT_SECONDS "$pull_timeout"
require_timeout_range AIGW_LINUX_IMAGE_LOCK_TIMEOUT_SECONDS "$lock_timeout"
[ -d "$out" ] || { echo "artifact directory does not exist: $out" >&2; exit 2; }
sh "$root/scripts/check-release-artifacts.sh" "$out" "$version" >/dev/null

run_with_timeout() {
  seconds=$1
  shift
  if command -v timeout >/dev/null 2>&1; then
    timeout "$seconds" "$@"
    return
  fi

  descendant_pids() {
    root_pid=$1
    ps -eo pid=,ppid= 2>/dev/null | awk -v root="$root_pid" '
      { parent[$1] = $2 }
      END {
        for (candidate in parent) {
          ancestor = candidate
          while (ancestor in parent && parent[ancestor] != 0) {
            ancestor = parent[ancestor]
            if (ancestor == root) {
              print candidate
              break
            }
          }
        }
      }
    '
  }

  process_tree_pids() {
    process_tree_root=$1
    printf '%s\n' "$process_tree_root"
    descendant_pids "$process_tree_root"
  }

  surviving_process_tree_pids() {
    retained_process_list=$1
    printf '%s\n' "$retained_process_list" |
      while IFS= read -r retained_pid; do
        case "$retained_pid" in
          ''|*[!0-9]*) continue ;;
        esac
        if kill -0 "$retained_pid" 2>/dev/null; then
          printf '%s\n' "$retained_pid"
          descendant_pids "$retained_pid"
        fi
      done |
      awk '!seen[$1]++'
  }

  signal_processes() {
    process_signal=$1
    process_list=$2
    printf '%s\n' "$process_list" |
      while IFS= read -r process_pid; do
        case "$process_pid" in
          ''|*[!0-9]*) continue ;;
        esac
        kill "-$process_signal" "$process_pid" 2>/dev/null || true
      done
  }

  "$@" &
  pid=$!
  elapsed=0
  while kill -0 "$pid" 2>/dev/null; do
    if [ "$elapsed" -ge "$seconds" ]; then
      # Retain the exact pre-TERM tree. A root that exits during the grace
      # period can no longer be used to rediscover its reparented descendants.
      retained_pids=$(process_tree_pids "$pid" | awk '!seen[$1]++')
      signal_processes TERM "$retained_pids"
      # Preserve a hard 300-second ceiling: the one-second TERM grace is only
      # available when it still fits below the admitted timeout maximum.
      if [ "$seconds" -lt 300 ]; then
        sleep 1
      fi
      # Signal only exact retained PIDs that are still live, plus their current
      # descendants. This avoids a broad process-group kill while covering
      # descendants spawned or reparented during TERM handling.
      kill_pids=$(surviving_process_tree_pids "$retained_pids")
      signal_processes KILL "$kill_pids"
      wait "$pid" 2>/dev/null || true
      return 124
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  wait "$pid"
}

image_lock_name() {
  printf '%s' "$1-$2" | tr '/:@' '___'
}

with_image_lock() {
  image=$1
  platform=$2
  shift 2
  (
    lock_dir="$shared_tmp_root/.image-lock-$(image_lock_name "$image" "$platform")"
    elapsed=0
    while ! mkdir "$lock_dir" 2>/dev/null; do
      if [ "$elapsed" -ge "$lock_timeout" ]; then
        echo "timed out waiting for Linux acceptance image lock: $image ($platform)" >&2
        exit 1
      fi
      sleep 1
      elapsed=$((elapsed + 1))
    done
    trap 'rmdir "$lock_dir" 2>/dev/null || true' EXIT HUP INT TERM
    "$@"
  )
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
cleanup() { rm -rf "$staged"; }
trap cleanup EXIT HUP INT TERM
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
