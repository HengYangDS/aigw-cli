#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
out=${1:?usage: test-linux-native-install.sh <dist-dir> <version>}
version=${2:?usage: test-linux-native-install.sh <dist-dir> <version>}
image=${AIGW_LINUX_ACCEPTANCE_IMAGE:-alpine:3.22}
alpine_repository=${AIGW_ALPINE_REPOSITORY:-https://dl-cdn.alpinelinux.org/alpine}
shared_tmp_root=${AIGW_DOCKER_SHARED_TMPDIR:-"$HOME/.cache/aigw/container-artifacts"}

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required for Linux native-install acceptance: $1" >&2
    exit 2
  }
}

require docker
[ -d "$out" ] || { echo "artifact directory does not exist: $out" >&2; exit 2; }
case "$alpine_repository" in
  https://*/alpine|http://*/alpine) ;;
  *) echo "AIGW_ALPINE_REPOSITORY must be an Alpine repository root ending in /alpine: $alpine_repository" >&2; exit 2 ;;
esac
sh "$root/scripts/check-release-artifacts.sh" "$out" "$version" >/dev/null

mkdir -p "$shared_tmp_root"
staged=$(mktemp -d "$shared_tmp_root/aigw-linux-native-install.XXXXXX")
cleanup() { rm -rf "$staged"; }
trap cleanup EXIT HUP INT TERM
cp "$out/aigw_${version}_linux_amd64.deb" "$staged/"
cp "$out/aigw_${version}_linux_amd64.rpm" "$staged/"

# The image must be Alpine x86_64. Its package tools identify the machine as
# musl-linux-amd64, while the package uses the Debian/RPM standard amd64 name.
# The AIGW payload is a static linux/amd64 binary, so
# --force-architecture/--ignorearch compensate only for that naming mismatch;
# they do not bypass a real CPU mismatch. This is a compatibility-harness
# result, not a substitute for Debian/Fedora CI.
docker run --platform linux/amd64 --rm --entrypoint /bin/sh -v "$staged:/artifacts:ro" \
  -e "AIGW_VERSION=$version" -e "AIGW_ALPINE_REPOSITORY=$alpine_repository" "$image" -exc '
    test -f /etc/alpine-release || { echo "acceptance image must be Alpine" >&2; exit 2; }
    release=$(cut -d. -f1-2 /etc/alpine-release)
    printf "%s/v%s/main\n%s/v%s/community\n" "$AIGW_ALPINE_REPOSITORY" "$release" "$AIGW_ALPINE_REPOSITORY" "$release" > /etc/apk/repositories
    apk add --no-cache --no-progress dpkg
    dpkg --force-architecture -i "/artifacts/aigw_${AIGW_VERSION}_linux_amd64.deb"
    test -x /usr/bin/aigw
    /usr/bin/aigw --version | grep -Fx "aigw version ${AIGW_VERSION}"
  '

docker run --platform linux/amd64 --rm --entrypoint /bin/sh -v "$staged:/artifacts:ro" \
  -e "AIGW_VERSION=$version" -e "AIGW_ALPINE_REPOSITORY=$alpine_repository" "$image" -exc '
    test -f /etc/alpine-release || { echo "acceptance image must be Alpine" >&2; exit 2; }
    release=$(cut -d. -f1-2 /etc/alpine-release)
    printf "%s/v%s/main\n%s/v%s/community\n" "$AIGW_ALPINE_REPOSITORY" "$release" "$AIGW_ALPINE_REPOSITORY" "$release" > /etc/apk/repositories
    apk add --no-cache --no-progress rpm
    rpm -ivh --nosignature --ignorearch "/artifacts/aigw_${AIGW_VERSION}_linux_amd64.rpm"
    test -x /usr/bin/aigw
    /usr/bin/aigw --version | grep -Fx "aigw version ${AIGW_VERSION}"
  '

echo "Linux amd64 native package install paths: OK (Alpine compatibility harness)"
