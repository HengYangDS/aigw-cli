#!/bin/sh
set -eu

source_binary=${1:?usage: test-installers.sh <aigw-binary>}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

[ -x "$source_binary" ] || {
  echo "source binary is not executable: $source_binary" >&2
  exit 2
}

unix_script="$root/scripts/install.sh"
powershell_script="$root/scripts/install.ps1"

if grep -n -- '--format[[:space:]]\+json' "$unix_script" "$powershell_script" >/dev/null; then
  echo "installers must use glab's current -F json output flag, not --format json" >&2
  exit 1
fi

for script in "$unix_script" "$powershell_script"; do
  if ! grep -Eq -- 'release list.*-F[[:space:]]+json' "$script"; then
    echo "missing glab -F json release-list invocation: $script" >&2
    exit 1
  fi
done
if ! grep -Eq -- 'release list.*--jq[[:space:]]+[^[:space:]]*\[0\]\.tag_name' "$unix_script"; then
  echo "Unix installer must ask glab to select the first tag instead of parsing JSON with sed" >&2
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
home="$tmp/home"
install_dir="$home/bin"
fake_bin="$tmp/fake-bin"
version=0.0.0-installer-smoke
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in darwin|linux) ;; *) echo "unsupported test OS: $os" >&2; exit 2 ;; esac
case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "unsupported test architecture: $arch" >&2; exit 2 ;; esac

archive="aigw_${version}_${os}_${arch}.tar.gz"
payload_dir="$tmp/payload/aigw_${version}_${os}_${arch}"
mkdir -p "$payload_dir" "$fake_bin"
cp "$source_binary" "$payload_dir/aigw"
chmod 755 "$payload_dir/aigw"
tar -czf "$tmp/$archive" -C "$tmp/payload" "$(basename "$payload_dir")"

if command -v sha256sum >/dev/null 2>&1; then
  checksum=$(sha256sum "$tmp/$archive" | awk '{print $1}')
else
  checksum=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
fi
printf '%s  %s\n' "$checksum" "$archive" > "$tmp/checksums.txt"

cat > "$fake_bin/glab" <<'SH'
#!/bin/sh
set -eu

case "${1-} ${2-}" in
  'release list')
    case " $* " in
      *' -F json '*) ;;
      *) echo "glab release list must receive -F json" >&2; exit 64 ;;
    esac
    case " $* " in
      *' --jq .[0].tag_name '*) ;;
      *) echo "glab release list must select the first tag with --jq" >&2; exit 64 ;;
    esac
    case " $* " in
      *' --format json '*) echo "obsolete --format json received" >&2; exit 64 ;;
    esac
    printf 'v%s\n' "$AIGW_TEST_VERSION"
    ;;
  'release download')
    asset=
    destination=
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --asset-name) asset=$2; shift 2 ;;
        --dir) destination=$2; shift 2 ;;
        *) shift ;;
      esac
    done
    [ -n "$asset" ] && [ -n "$destination" ] || {
      echo "incomplete glab release download invocation" >&2
      exit 64
    }
    case "$asset" in
      "$AIGW_TEST_ARCHIVE") cp "$AIGW_TEST_ARCHIVE_PATH" "$destination/$asset" ;;
      checksums.txt) cp "$AIGW_TEST_CHECKSUM_PATH" "$destination/checksums.txt" ;;
      *) echo "unexpected requested asset: $asset" >&2; exit 64 ;;
    esac
    ;;
  *)
    echo "unexpected glab invocation: $*" >&2
    exit 64
    ;;
esac
SH
chmod 755 "$fake_bin/glab"

unset AIGW_SOURCE_BINARY
env \
  HOME="$home" \
  SHELL=/bin/sh \
  PATH="$fake_bin:$PATH" \
  AIGW_VERSION=latest \
  AIGW_INSTALL_DIR="$install_dir" \
  AIGW_TEST_VERSION="$version" \
  AIGW_TEST_ARCHIVE="$archive" \
  AIGW_TEST_ARCHIVE_PATH="$tmp/$archive" \
  AIGW_TEST_CHECKSUM_PATH="$tmp/checksums.txt" \
  /bin/sh "$unix_script"

installed="$install_dir/aigw"
[ -x "$installed" ] || { echo "release installer did not produce an executable" >&2; exit 1; }
"$installed" --version >/dev/null

echo "release installer latest-version discovery: OK"
