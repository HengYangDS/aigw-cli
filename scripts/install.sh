#!/bin/sh
set -eu

project_url=${AIGW_PROJECT_URL:-http://192.168.64.101:18086/dig/misc/agentic-third-party-api/aigw-cli}
project_path=dig/misc/agentic-third-party-api/aigw-cli
version=${AIGW_VERSION:-latest}
install_dir=${AIGW_INSTALL_DIR:-"$HOME/.local/bin"}
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

mkdir -p "$install_dir"

shell_profile() {
  shell_name=$(basename "${SHELL:-sh}")
  case "$shell_name" in
    zsh) echo "$HOME/.zshrc" ;;
    bash) [ -f "$HOME/.bash_profile" ] && echo "$HOME/.bash_profile" || echo "$HOME/.bashrc" ;;
    fish) mkdir -p "$HOME/.config/fish"; echo "$HOME/.config/fish/config.fish" ;;
    *) echo "$HOME/.profile" ;;
  esac
}

ensure_path() {
  case ":$PATH:" in *":$install_dir:"*) return 0 ;; esac
  profile=$(shell_profile)
  mkdir -p "$(dirname -- "$profile")"
  begin="# >>> AIGW PATH >>>"
  end="# <<< AIGW PATH <<<"
  tmp="$profile.aigw.$$"
  if [ -f "$profile" ]; then
    awk -v begin="$begin" -v end="$end" '
      $0 == begin {skip=1; next}
      $0 == end {skip=0; next}
      skip != 1 {print}
    ' "$profile" > "$tmp"
  else
    : > "$tmp"
  fi
  shell_name=$(basename "${SHELL:-sh}")
  {
    printf '\n%s\n' "$begin"
    if [ "$shell_name" = fish ]; then
      printf 'fish_add_path %s\n' "$install_dir"
    else
      printf 'export PATH="%s:$PATH"\n' "$install_dir"
    fi
    printf '%s\n' "$end"
  } >> "$tmp"
  mv "$tmp" "$profile"
  echo "PATH updated in $profile. Open a new terminal, or run: export PATH=\"$install_dir:\$PATH\""
}

gitlab_api_url() {
  encoded_path=$(printf '%s' "$project_path" | sed 's#/#%2F#g')
  printf '%s/api/v4/projects/%s/releases/permalink/latest' "${AIGW_GL_HOST:-http://192.168.64.101:18086}" "$encoded_path"
}

gitlab_token_curl_config() {
  config=$1
  case "$GITLAB_TOKEN" in
    *"
"*|*""*)
      echo "GITLAB_TOKEN contains a control character" >&2
      return 2
      ;;
  esac
  umask 077
  escaped_token=$(printf '%s' "$GITLAB_TOKEN" | sed 's/\\/\\\\/g; s/"/\\"/g')
  printf 'header = "PRIVATE-TOKEN: %s"\n' "$escaped_token" > "$config"
  chmod 600 "$config"
}

# AIGW_SOURCE_BINARY is an explicit local-test seam. Release downloads still
# require authenticated retrieval and checksum validation below.
if [ -n "${AIGW_SOURCE_BINARY:-}" ]; then
  [ -f "$AIGW_SOURCE_BINARY" ] && [ -x "$AIGW_SOURCE_BINARY" ] || {
    echo "AIGW_SOURCE_BINARY must reference an executable local file" >&2
    exit 2
  }
  source_binary=$AIGW_SOURCE_BINARY
elif [ -x "$script_dir/aigw" ]; then
  source_binary="$script_dir/aigw"
else
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)
  case "$os" in darwin|linux) ;; *) echo "unsupported OS: $os" >&2; exit 2 ;; esac
  case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "unsupported architecture: $arch" >&2; exit 2 ;; esac
  if [ "$version" = latest ]; then
    if command -v glab >/dev/null 2>&1; then
      version=$(GL_HOST=${AIGW_GL_HOST:-http://192.168.64.101:18086} glab release list -R "$project_path" --per-page 1 -F json --jq '.[0].tag_name')
    elif [ -n "${GITLAB_TOKEN:-}" ]; then
      tmp=$(mktemp -d)
      trap 'rm -rf "$tmp"' EXIT HUP INT TERM
      curl_config="$tmp/curl.conf"
      gitlab_token_curl_config "$curl_config"
      curl -fsSL --config "$curl_config" "$(gitlab_api_url)" -o "$tmp/latest-release.json"
      version=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp/latest-release.json" | head -n 1)
    else
      echo "latest private release requires authenticated glab or GITLAB_TOKEN; set AIGW_VERSION to install a known tag" >&2
      exit 2
    fi
    [ -n "$version" ] || { echo "no release found" >&2; exit 2; }
  fi
  clean_version=${version#v}
  archive="aigw_${clean_version}_${os}_${arch}.tar.gz"
  if [ -z "${tmp:-}" ]; then
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT HUP INT TERM
  fi
  tag=$version
  case "$tag" in v*) ;; *) tag="v$tag" ;; esac
  url="$project_url/-/releases/$tag/downloads/$archive"
  if command -v glab >/dev/null 2>&1; then
    GL_HOST=${AIGW_GL_HOST:-http://192.168.64.101:18086} glab release download "$tag" -R "$project_path" --asset-name "$archive" --dir "$tmp"
    GL_HOST=${AIGW_GL_HOST:-http://192.168.64.101:18086} glab release download "$tag" -R "$project_path" --asset-name checksums.txt --dir "$tmp"
  elif [ -n "${GITLAB_TOKEN:-}" ]; then
    curl_config="$tmp/curl.conf"
    gitlab_token_curl_config "$curl_config"
    curl -fsSL --config "$curl_config" "$url" -o "$tmp/$archive"
    curl -fsSL --config "$curl_config" "$project_url/-/releases/$tag/downloads/checksums.txt" -o "$tmp/checksums.txt"
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
ensure_path

echo "Installed $install_dir/aigw"
echo "Next: aigw setup"
