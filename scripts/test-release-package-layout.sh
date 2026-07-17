#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
out=${1:-}
version=${2:-}
created_out=0
work=$(mktemp -d)
trap 'rm -rf "$work"; [ "$created_out" = 0 ] || rm -rf "$out"' EXIT HUP INT TERM

if [ -z "$out" ] || [ -z "$version" ]; then
  version=0.1.0-package-test
  out=$(mktemp -d)
  created_out=1
fi

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required for package-layout test: $1" >&2
    exit 2
  }
}

for command in file tar unzip ar bsdtar msiextract msiinfo pkgutil lipo; do
  require "$command"
done

if [ "$created_out" = 1 ]; then
  for command in go nfpm wixl msibuild xar python3; do
    require "$command"
  done
  SOURCE_DATE_EPOCH=1784246400 \
    AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example.test \
    AIGW_GITLAB_RELEASE_REPOSITORY=example-group/aigw-cli \
    AIGW_GITHUB_RELEASE_ORIGIN=https://github.com \
    AIGW_GITHUB_RELEASE_REPOSITORY=example-owner/aigw-cli \
    AIGW_REQUIRE_FULL_MATRIX=1 sh "$root/scripts/package.sh" "$version" "$out" >/dev/null
fi
[ -d "$out" ] || { echo "artifact directory does not exist: $out" >&2; exit 2; }
sh "$root/scripts/check-release-artifacts.sh" "$out" "$version" >/dev/null
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$out" && sha256sum -c checksums.txt >/dev/null)
else
  (cd "$out" && shasum -a 256 -c checksums.txt >/dev/null)
fi

for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}
  arch=${target#*/}
  binary=aigw
  extension=tar.gz
  [ "$os" = windows ] && { binary=aigw.exe; extension=zip; }
  archive="$out/aigw_${version}_${os}_${arch}.${extension}"
  stage="$work/$os-$arch"
  mkdir -p "$stage"
  if [ "$os" = windows ]; then
    unzip -qq "$archive" -d "$stage"
  else
    tar -xzf "$archive" -C "$stage"
  fi
  payload_root="$stage/aigw_${version}_${os}_${arch}"
  payload="$payload_root/$binary"
  [ -x "$payload" ] || { echo "portable package missing executable: $os/$arch" >&2; exit 1; }
  cmp -s "$root/LICENSE" "$payload_root/LICENSE" || {
    echo "portable package license file differs from repository MIT license: $os/$arch" >&2
    exit 1
  }
  if [ "$os" != windows ]; then
    installer="$stage/aigw_${version}_${os}_${arch}/install.sh"
    [ -x "$installer" ] || { echo "portable package installer is not executable: $os/$arch" >&2; exit 1; }
  fi
  description=$(file "$payload")
  case "$target:$description" in
    darwin/amd64:*x86_64*) ;;
    darwin/arm64:*arm64*) ;;
    linux/amd64:*x86-64*) ;;
    linux/arm64:*aarch64*) ;;
    windows/amd64:*x86-64*) ;;
    windows/arm64:*[Aa]arch64*) ;;
    *) echo "unexpected portable executable architecture for $target: $description" >&2; exit 1 ;;
  esac
done

pkg="$out/aigw_${version}_darwin_universal.pkg"
pkgutil --expand-full "$pkg" "$work/pkg" >/dev/null
universal="$work/pkg/aigw-component.pkg/Payload/usr/local/bin/aigw"
[ -x "$universal" ] || { echo "macOS pkg missing /usr/local/bin/aigw" >&2; exit 1; }
case "$(lipo -archs "$universal")" in
  *x86_64*arm64*|*arm64*x86_64*) ;;
  *) echo "macOS pkg binary is not universal" >&2; exit 1 ;;
esac

macos_uninstaller="$work/pkg/aigw-component.pkg/Payload/usr/local/libexec/aigw/uninstall"
[ -x "$macos_uninstaller" ] || { echo "macOS pkg missing its owned uninstaller" >&2; exit 1; }
cmp -s "$root/packaging/macos/aigw-uninstall" "$macos_uninstaller" || {
  echo "macOS pkg uninstaller differs from the canonical owned uninstaller" >&2
  exit 1
}

for arch in amd64 arm64; do
  deb="$out/aigw_${version}_linux_${arch}.deb"
  control=$(ar t "$deb" | awk '/^control\.tar/ {print; exit}')
  data=$(ar t "$deb" | awk '/^data\.tar/ {print; exit}')
  [ -n "$control" ] && [ -n "$data" ] || { echo "invalid deb structure: $arch" >&2; exit 1; }
  ar p "$deb" "$control" | bsdtar -xOf - ./control > "$work/control-$arch"
  grep -Fx "Architecture: $arch" "$work/control-$arch" >/dev/null || { echo "deb architecture mismatch: $arch" >&2; exit 1; }
  ar p "$deb" "$data" | bsdtar -xOf - ./usr/bin/aigw > "$work/deb-$arch"
  ar p "$deb" "$data" | bsdtar -xOf - ./usr/share/doc/aigw/copyright > "$work/deb-copyright-$arch"
  cmp -s "$root/LICENSE" "$work/deb-copyright-$arch" || {
    echo "deb package license file differs from repository MIT license: $arch" >&2
    exit 1
  }
  description=$(file "$work/deb-$arch")
  case "$arch:$description" in
    amd64:*x86-64*) ;;
    arm64:*aarch64*) ;;
    *) echo "unexpected deb executable architecture for $arch: $description" >&2; exit 1 ;;
  esac

  rpm="$out/aigw_${version}_linux_${arch}.rpm"
  bsdtar -tf "$rpm" | grep -Fx /usr/bin/aigw >/dev/null || { echo "rpm missing /usr/bin/aigw: $arch" >&2; exit 1; }
  bsdtar -tf "$rpm" | grep -Fx /usr/share/doc/aigw/copyright >/dev/null || { echo "rpm missing MIT license file: $arch" >&2; exit 1; }
  bsdtar -xOf "$rpm" /usr/bin/aigw > "$work/rpm-$arch"
  bsdtar -xOf "$rpm" /usr/share/doc/aigw/copyright > "$work/rpm-copyright-$arch"
  cmp -s "$root/LICENSE" "$work/rpm-copyright-$arch" || {
    echo "rpm package license file differs from repository MIT license: $arch" >&2
    exit 1
  }
  description=$(file "$work/rpm-$arch")
  case "$arch:$description" in
    amd64:*x86-64*) ;;
    arm64:*aarch64*) ;;
    *) echo "unexpected rpm executable architecture for $arch: $description" >&2; exit 1 ;;
  esac
done

# shellcheck source=msi-version.sh
. "$root/scripts/msi-version.sh"

msi_component_guid() {
  case "$1:$2" in
    amd64:AigwExe) echo '{F32B31D8-2CCE-4D50-959C-9290F352F0C5}' ;;
    amd64:AigwPath) echo '{8361E00B-0AC1-42E7-A16A-3354BF84CEAE}' ;;
    arm64:AigwExe) echo '{4D557EAF-95D3-4B6B-98ED-D29DA8C754FA}' ;;
    arm64:AigwPath) echo '{511FB0FA-7943-45F7-B9EB-A9C203464338}' ;;
    *) echo "unknown expected MSI component: $1/$2" >&2; exit 2 ;;
  esac
}

for arch in amd64 arm64; do
  msi="$out/aigw_${version}_windows_${arch}.msi"
  stage="$work/msi-$arch"
  product_version=$(msiinfo export "$msi" Property | awk -F '	' 'NR > 3 && $1 == "ProductVersion" {print $2; exit}' | tr -d '\r')
  expected_product_version=$(msi_version)
  [ "$product_version" = "$expected_product_version" ] || {
    echo "unexpected MSI ProductVersion for $arch: $product_version" >&2
    exit 1
  }
  template=$(msiinfo suminfo "$msi" | awk -F': ' '$1 == "Template" {print $2; exit}')
  expected_template=x64
  [ "$arch" = arm64 ] && expected_template=Arm64
  [ "$template" = "${expected_template};1033" ] || {
    echo "unexpected MSI platform template for $arch: $template" >&2
    exit 1
  }
  subject=$(msiinfo suminfo "$msi" | awk -F': ' '$1 == "Subject" {print $2; exit}')
  [ "$subject" = "AIGW CLI" ] || {
    echo "unexpected MSI subject for $arch: $subject" >&2
    exit 1
  }
  component_ids=$(msiinfo export "$msi" Component | awk -F '	' 'NR > 3 && $1 ~ /^Aigw(Exe|Path)$/ {print $1 "=" $2}')
  for component in AigwExe AigwPath; do
    expected=$(msi_component_guid "$arch" "$component")
    actual=$(printf '%s\n' "$component_ids" | awk -F= -v component="$component" '$1 == component {print $2; exit}')
    [ -n "$actual" ] || { echo "MSI missing $component component GUID: $arch" >&2; exit 1; }
    [ "$actual" = "$expected" ] || {
      echo "unexpected MSI $component component GUID for $arch: $actual" >&2
      exit 1
    }
  done
  environment=$(msiinfo export "$msi" Environment | awk -F '	' 'NR > 3 && $2 == "=PATH" && $4 == "AigwPath\r" {print $1 "\t" $2 "\t" $3 "\t" $4; exit}' | tr -d '\r')
  expected_environment=$(go run "$root/tools/releaseid" \
    -namespace 6ba7b814-9dad-11d1-80b4-00c04fd430c8 \
    -name "aigw/environment/$version/windows/$arch")
  expected_environment=$(printf '%s\t=PATH\t[~];[INSTALLBINFOLDER]\tAigwPath' "$expected_environment")
  [ "$environment" = "$expected_environment" ] || {
    echo "unexpected MSI PATH environment entry for $arch: ${environment:-missing}" >&2
    exit 1
  }
  msiextract -C "$stage" "$msi" >/dev/null
  payload=$(find "$stage" -type f -name aigw.exe -print -quit)
  [ -n "$payload" ] || { echo "MSI missing aigw.exe: $arch" >&2; exit 1; }
  description=$(file "$payload")
  case "$arch:$description" in
    amd64:*x86-64*) ;;
    arm64:*[Aa]arch64*) ;;
    *) echo "unexpected MSI executable architecture for $arch: $description" >&2; exit 1 ;;
  esac
done

echo "release package layout: OK"
