#!/bin/sh
set -eu

project_url=${AIGW_PROJECT_URL:-http://192.168.64.101:18086/dig/misc/agentic-third-party-api/aigw-cli}
version=${AIGW_VERSION:-latest}
install_dir=${AIGW_INSTALL_DIR:-"$HOME/.local/bin"}
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

mkdir -p "$install_dir"

if [ -x "$script_dir/aigw" ]; then
  source_binary="$script_dir/aigw"
else
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)
  case "$os" in darwin|linux) ;; *) echo "unsupported OS: $os" >&2; exit 2 ;; esac
  case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "unsupported architecture: $arch" >&2; exit 2 ;; esac
  if [ "$version" = latest ]; then
    if ! command -v glab >/dev/null 2>&1; then
      echo "glab is required to resolve the latest release of this private project; set AIGW_VERSION to a tag or install glab" >&2
      exit 2
    fi
    version=$(GL_HOST=${AIGW_GL_HOST:-http://192.168.64.101:18086} glab release list -R dig/misc/agentic-third-party-api/aigw-cli --per-page 1 --format json | sed -n 's/.*"tag_name":"\([^"]*\)".*/\1/p')
    [ -n "$version" ] || { echo "no release found" >&2; exit 2; }
  fi
  clean_version=${version#v}
  archive="aigw_${clean_version}_${os}_${arch}.tar.gz"
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT HUP INT TERM
  tag=$version
  case "$tag" in v*) ;; *) tag="v$tag" ;; esac
  url="$project_url/-/releases/$tag/downloads/$archive"
  if command -v glab >/dev/null 2>&1; then
    GL_HOST=${AIGW_GL_HOST:-http://192.168.64.101:18086} glab release download "$tag" \
      -R dig/misc/agentic-third-party-api/aigw-cli --asset-name "$archive" --dir "$tmp"
    GL_HOST=${AIGW_GL_HOST:-http://192.168.64.101:18086} glab release download "$tag" \
      -R dig/misc/agentic-third-party-api/aigw-cli --asset-name checksums.txt --dir "$tmp"
  elif [ -n "${GITLAB_TOKEN:-}" ]; then
    curl -fsSL -H "PRIVATE-TOKEN: $GITLAB_TOKEN" "$url" -o "$tmp/$archive"
    curl -fsSL -H "PRIVATE-TOKEN: $GITLAB_TOKEN" \
      "$project_url/-/releases/$tag/downloads/checksums.txt" -o "$tmp/checksums.txt"
  else
    echo "private release download requires authenticated glab or GITLAB_TOKEN" >&2
    exit 2
  fi
  expected=$(awk -v name="$archive" '$2 == name || $2 == "./" name {print $1; exit}' "$tmp/checksums.txt")
  [ -n "$expected" ] || { echo "checksum entry missing for $archive" >&2; exit 2; }
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
  else
    actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
  fi
  [ "$actual" = "$expected" ] || { echo "SHA-256 mismatch for $archive" >&2; exit 2; }
  tar -xzf "$tmp/$archive" -C "$tmp"
  source_binary="$tmp/aigw_${clean_version}_${os}_${arch}/aigw"
fi

tmp_binary="$install_dir/.aigw.new.$$"
cp "$source_binary" "$tmp_binary"
chmod 755 "$tmp_binary"
mv "$tmp_binary" "$install_dir/aigw"

echo "Installed $install_dir/aigw"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "Add $install_dir to PATH, then run: aigw setup" ;;
esac
