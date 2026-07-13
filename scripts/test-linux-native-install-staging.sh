#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
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

echo "Linux native-install shared-staging contract: OK"
