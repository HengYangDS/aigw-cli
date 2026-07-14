#!/bin/sh
set -eu

version=${1:-0.1.0-dev}
out=${2:-dist}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
module=$(awk 'NR == 1 && $1 == "module" { print $2; exit }' "$root/go.mod")
release_provider=${AIGW_RELEASE_PROVIDER:-gitlab}
release_host=${AIGW_RELEASE_HOST:-}
release_project=${AIGW_RELEASE_PROJECT:-}
release_mirror_provider=${AIGW_RELEASE_MIRROR_PROVIDER:-}
release_mirror_host=${AIGW_RELEASE_MIRROR_HOST:-}
release_mirror_project=${AIGW_RELEASE_MIRROR_PROJECT:-}

"$root/scripts/check-package-safety.sh"
"$root/scripts/check-retired-residue.sh"

case "$version" in
  *[!0-9A-Za-z._-]*) echo "invalid version: $version" >&2; exit 2 ;;
esac

rm -rf "$out"
mkdir -p "$out"
out_abs=$(CDPATH= cd -- "$out" && pwd)
build_root=$(mktemp -d)
trap 'rm -rf "$build_root"' EXIT HUP INT TERM

build_binary() {
  goos=$1
  goarch=$2
  channel=$3
  dest=$4
  printf 'building %s/%s (%s)\n' "$goos" "$goarch" "$channel"
  mkdir -p "$(dirname -- "$dest")"
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
    -ldflags "-s -w -X ${module}/internal/cli.Version=$version -X ${module}/internal/selfupdate.InstallChannel=$channel -X ${module}/internal/selfupdate.BuildReleaseProvider=$release_provider -X ${module}/internal/selfupdate.BuildReleaseHost=$release_host -X ${module}/internal/selfupdate.BuildReleaseProject=$release_project -X ${module}/internal/selfupdate.BuildReleaseMirrorProvider=$release_mirror_provider -X ${module}/internal/selfupdate.BuildReleaseMirrorHost=$release_mirror_host -X ${module}/internal/selfupdate.BuildReleaseMirrorProject=$release_mirror_project" \
    -o "$dest" ./cmd/aigw
}

archive_portable() {
  goos=$1
  goarch=$2
  binary=aigw
  [ "$goos" = windows ] && binary=aigw.exe
  name="aigw_${version}_${goos}_${goarch}"
  stage="$build_root/$name"
  mkdir -p "$stage"
  build_binary "$goos" "$goarch" portable "$stage/$binary"
  cp "$root/README.md" "$stage/"
  cp "$root/scripts/install.sh" "$root/scripts/uninstall.sh" "$stage/"
  cp "$root/scripts/install.ps1" "$root/scripts/uninstall.ps1" "$stage/"
  chmod 755 "$stage/install.sh" "$stage/uninstall.sh"
  if [ "$goos" = windows ]; then
    (cd "$build_root" && zip -qr "$out_abs/$name.zip" "$name")
  else
    (cd "$build_root" && tar --no-xattrs -czf "$out_abs/$name.tar.gz" "$name")
  fi
}

build_portable_archives() {
  for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
    archive_portable "${target%/*}" "${target#*/}"
  done
}

build_macos_pkg() {
  if ! command -v lipo >/dev/null 2>&1 || ! command -v pkgbuild >/dev/null 2>&1 || ! command -v productbuild >/dev/null 2>&1; then
    echo "skipping macOS .pkg: lipo, pkgbuild, or productbuild not available" >&2
    return 0
  fi
  pkg_root="$build_root/pkgroot"
  scripts_dir="$build_root/pkg-scripts"
  component="$build_root/aigw-component.pkg"
  universal="$build_root/aigw-universal"
  rm -rf "$pkg_root" "$scripts_dir"
  mkdir -p "$pkg_root/usr/local/bin" "$scripts_dir"
  build_binary darwin amd64 pkg "$build_root/aigw-darwin-amd64"
  build_binary darwin arm64 pkg "$build_root/aigw-darwin-arm64"
  lipo -create -output "$universal" "$build_root/aigw-darwin-amd64" "$build_root/aigw-darwin-arm64"
  cp "$universal" "$pkg_root/usr/local/bin/aigw"
  chmod 755 "$pkg_root/usr/local/bin/aigw"
  cp "$root/packaging/macos/aigw-postinstall" "$scripts_dir/postinstall"
  chmod 755 "$scripts_dir/postinstall"
  pkgbuild --root "$pkg_root" --scripts "$scripts_dir" --identifier "dig.aigw.cli" --version "$version" --install-location / "$component" >/dev/null
  productbuild --package "$component" "$out_abs/aigw_${version}_darwin_universal.pkg" >/dev/null
}

nfpm_arch() {
  case "$1" in
    amd64) echo amd64 ;;
    arm64) echo arm64 ;;
    *) echo "$1" ;;
  esac
}

sed_escape() {
  printf '%s' "$1" | sed 's/[&|\\]/\\&/g'
}

render_nfpm_config() {
  src=$1
  dst=$2
  arch=$3
  pkg_stage=$4
  sed \
    -e "s|\${AIGW_NFPM_ARCH}|$(sed_escape "$arch")|g" \
    -e "s|\${AIGW_VERSION}|$(sed_escape "$version")|g" \
    -e "s|\${AIGW_PACKAGE_ROOT}|$(sed_escape "$pkg_stage")|g" \
    -e "s|\${AIGW_REPO_ROOT}|$(sed_escape "$root")|g" \
    "$src" > "$dst"
}

build_linux_packages() {
  if ! command -v nfpm >/dev/null 2>&1; then
    echo "skipping Linux .deb/.rpm: nfpm not available" >&2
    return 0
  fi
  for arch in amd64 arm64; do
    for channel in deb rpm; do
      pkg_stage="$build_root/linux-${channel}-${arch}"
      rm -rf "$pkg_stage"
      mkdir -p "$pkg_stage"
      build_binary linux "$arch" "$channel" "$pkg_stage/aigw"
      cat > "$pkg_stage/postinstall.sh" <<'EOS'
#!/bin/sh
set -eu
cat <<'MSG'
AIGW installed to /usr/bin/aigw.
Run: aigw setup
Claude shim is created later in AIGW's own data directory by the target user's aigw setup/repair command.
MSG
EOS
      chmod 755 "$pkg_stage/postinstall.sh"
      rendered="$pkg_stage/nfpm.yaml"
      render_nfpm_config "$root/packaging/linux/nfpm.yaml" "$rendered" "$(nfpm_arch "$arch")" "$pkg_stage"
      nfpm package -f "$rendered" -p "$channel" -t "$out_abs/aigw_${version}_linux_${arch}.${channel}" >/dev/null
    done
  done
}

wix_arch() {
  case "$1" in
    amd64) echo x64 ;;
    arm64) echo arm64 ;;
    *) echo "$1" ;;
  esac
}

msi_component_guid() {
  arch=$1
  component=$2
  case "$arch:$component" in
    amd64:AigwExe) echo F32B31D8-2CCE-4D50-959C-9290F352F0C5 ;;
    amd64:AigwPath) echo 8361E00B-0AC1-42E7-A16A-3354BF84CEAE ;;
    arm64:AigwExe) echo 4D557EAF-95D3-4B6B-98ED-D29DA8C754FA ;;
    arm64:AigwPath) echo 511FB0FA-7943-45F7-B9EB-A9C203464338 ;;
    *) echo "unknown MSI component GUID: $arch/$component" >&2; exit 2 ;;
  esac
}

# shellcheck source=msi-version.sh
. "$root/scripts/msi-version.sh"

build_windows_msi() {
  if ! command -v wixl >/dev/null 2>&1; then
    echo "skipping Windows .msi: wixl not available" >&2
    return 0
  fi
  if ! command -v uuidgen >/dev/null 2>&1; then
    echo "skipping Windows .msi: uuidgen not available" >&2
    return 0
  fi
  for arch in amd64 arm64; do
    wix_target=$(wix_arch "$arch")
    msi_stage="$build_root/msi-${arch}"
    rm -rf "$msi_stage"
    mkdir -p "$msi_stage"
    build_binary windows "$arch" msi "$msi_stage/aigw.exe"
    exe_guid=$(msi_component_guid "$arch" AigwExe)
    path_guid=$(msi_component_guid "$arch" AigwPath)
    if [ "$arch" = arm64 ] && ! wixl -a arm64 -E "$root/packaging/windows/aigw.wxs" >/dev/null 2>"$msi_stage/arm64-probe.err"; then
      wix_target=x64
    fi
    if wixl -a "$wix_target" -D "ProductVersion=$(msi_version)" -D "SourceDir=$msi_stage" \
      -D "AigwExeGuid=$exe_guid" -D "AigwPathGuid=$path_guid" \
      -o "$out_abs/aigw_${version}_windows_${arch}.msi" "$root/packaging/windows/aigw.wxs" >/dev/null 2>"$msi_stage/wixl.err"; then
      if [ "$arch" = arm64 ] && [ "$wix_target" = x64 ]; then
        if command -v msibuild >/dev/null 2>&1; then
          msibuild "$out_abs/aigw_${version}_windows_${arch}.msi" \
            -s "Installation Database" "DIG" "Arm64;1033" "$(uuidgen | tr '[:lower:]' '[:upper:]')" >/dev/null
        else
          rm -f "$out_abs/aigw_${version}_windows_${arch}.msi"
          echo "skipping Windows arm64 .msi: wixl lacks arm64 and msibuild is unavailable; portable zip is still generated" >&2
        fi
      fi
    else
      rm -f "$out_abs/aigw_${version}_windows_${arch}.msi"
      if [ "$arch" = arm64 ]; then
        echo "skipping Windows arm64 .msi: installed wixl does not support -a arm64; portable zip is still generated" >&2
      else
        cat "$msi_stage/wixl.err" >&2
        return 1
      fi
    fi
  done
}

verify_binary_arches() {
  if ! command -v file >/dev/null 2>&1; then
    echo "skipping binary architecture verification: file not available" >&2
    return 0
  fi
  verify_dir="$build_root/verify"
  rm -rf "$verify_dir"
  mkdir -p "$verify_dir"
  for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
    goos=${target%/*}
    goarch=${target#*/}
    binary=aigw
    [ "$goos" = windows ] && binary=aigw.exe
    build_binary "$goos" "$goarch" portable "$verify_dir/${goos}-${goarch}-$binary"
    desc=$(file "$verify_dir/${goos}-${goarch}-$binary")
    case "$target:$desc" in
      darwin/amd64:*x86_64*) ;;
      darwin/arm64:*arm64*) ;;
      linux/amd64:*x86-64*) ;;
      linux/arm64:*aarch64*) ;;
      windows/amd64:*x86-64*) ;;
      windows/arm64:*[Aa]arch64*) ;;
      *) echo "unexpected binary architecture for $target: $desc" >&2; exit 1 ;;
    esac
  done
}

write_checksums() {
  (cd "$out_abs" && {
    files=$(find . -maxdepth 1 -type f ! -name checksums.txt | sort | sed 's#^./##')
    if [ -z "$files" ]; then
      echo "no release artifacts produced" >&2
      exit 1
    fi
    if command -v sha256sum >/dev/null 2>&1; then
      # shellcheck disable=SC2086
      sha256sum $files > checksums.txt
    else
      # shellcheck disable=SC2086
      shasum -a 256 $files > checksums.txt
    fi
  })
}

build_portable_archives
build_macos_pkg
build_linux_packages
build_windows_msi
verify_binary_arches
go run ./tools/sbom -version "$version" > "$out_abs/aigw_${version}.spdx.json"
write_checksums
if [ "${AIGW_REQUIRE_FULL_MATRIX:-0}" = "1" ]; then
  "$root/scripts/check-release-artifacts.sh" "$out_abs" "$version"
fi
printf 'release artifacts written to %s\n' "$out_abs"
