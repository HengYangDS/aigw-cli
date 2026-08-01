#!/bin/sh
set -eu

version=${1:-0.1.0-dev}
out=${2:-dist}
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
module=$(awk 'NR == 1 && $1 == "module" { print $2; exit }' "$root/go.mod")
go_version=$(awk '$1 == "go" { print $2; exit }' "$root/go.mod")
source_date_epoch=${SOURCE_DATE_EPOCH:-}
requested_go_toolchain=${AIGW_RELEASE_GO_TOOLCHAIN:-}
package_maintainer=${AIGW_PACKAGE_MAINTAINER:-AIGW CLI Contributors}
package_homepage=${AIGW_PACKAGE_HOMEPAGE:-}

[ -n "$go_version" ] || { echo "release toolchain: go.mod has no Go version" >&2; exit 2; }
go_toolchain="go$go_version"

case "$requested_go_toolchain" in
  ''|"$go_toolchain") ;;
  *) echo "AIGW_RELEASE_GO_TOOLCHAIN conflicts with go.mod: expected $go_toolchain" >&2; exit 2 ;;
esac

export GOTOOLCHAIN="$go_toolchain"

release_source_exports=$(python3 "$root/scripts/release/lib/resolve-release-forge-sources.py" \
  --environment --shell)
# The resolver emits only validated shell-quoted values from explicit release
# execution inputs. This avoids baking an adopter or Forge into product source.
# shellcheck disable=SC2086
eval "$release_source_exports"
gitlab_origin=$AIGW_GITLAB_RELEASE_ORIGIN
gitlab_repository=$AIGW_GITLAB_RELEASE_REPOSITORY
github_origin=$AIGW_GITHUB_RELEASE_ORIGIN
github_repository=$AIGW_GITHUB_RELEASE_REPOSITORY

tuple_count() {
  count=0
  for value in "$@"; do
    [ -z "$value" ] || count=$((count + 1))
  done
  printf '%s\n' "$count"
}

validate_release_source() {
  provider=$1
  origin=$2
  repository=$3
  count=$(tuple_count "$origin" "$repository")
  [ "$count" -eq 0 ] || [ "$count" -eq 2 ] || {
    echo "$provider release configuration must include both origin and repository" >&2
    exit 2
  }
  case "$origin$repository" in
    *[![:print:]]*|*[[:space:]]*)
      echo "$provider release configuration must not contain whitespace or control characters" >&2
      exit 2
      ;;
  esac
}

validate_release_source GitLab "$gitlab_origin" "$gitlab_repository"
validate_release_source GitHub "$github_origin" "$github_repository"
[ -n "$package_homepage" ] || {
  echo "AIGW_PACKAGE_HOMEPAGE must be supplied by the release context" >&2
  exit 2
}
case "$package_homepage" in
  https://*) ;;
  *) echo "AIGW_PACKAGE_HOMEPAGE must be an https URL" >&2; exit 2 ;;
esac

"$root/scripts/checks/release/check-package-safety.sh"
"$root/scripts/checks/release/check-retired-residue.sh"
python3 "$root/scripts/checks/governance/check-text-layout.py"
"$root/scripts/checks/release/check-release-toolchain.sh"

case "$version" in
  *[!0-9A-Za-z._-]*) echo "invalid version: $version" >&2; exit 2 ;;
esac
case "$source_date_epoch" in
  ''|*[!0-9]*) echo "SOURCE_DATE_EPOCH must be a non-negative Unix epoch" >&2; exit 2 ;;
esac
source_date_touch=$(python3 - "$source_date_epoch" <<'PYTHON'
import datetime as dt
import sys

print(dt.datetime.fromtimestamp(int(sys.argv[1]), tz=dt.timezone.utc).strftime("%Y%m%d%H%M.%S"))
PYTHON
)

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
  (cd "$root" && CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X ${module}/internal/cli.Version=$version -X ${module}/internal/selfupdate.InstallChannel=$channel -X ${module}/internal/selfupdate.BuildGitLabReleaseOrigin=$gitlab_origin -X ${module}/internal/selfupdate.BuildGitLabReleaseRepository=$gitlab_repository -X ${module}/internal/selfupdate.BuildGitHubReleaseOrigin=$github_origin -X ${module}/internal/selfupdate.BuildGitHubReleaseRepository=$github_repository" \
    -o "$dest" ./cmd/aigw)
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
  cp "$root/README.md" "$root/LICENSE" "$stage/"
  cp "$root/scripts/install/install.sh" "$root/scripts/install/uninstall.sh" "$stage/"
  cp "$root/scripts/install/install.ps1" "$root/scripts/install/uninstall.ps1" "$stage/"
  chmod 755 "$stage/install.sh" "$stage/uninstall.sh"
  format=tar.gz
  archive="$out_abs/$name.tar.gz"
  [ "$goos" = windows ] && { format=zip; archive="$out_abs/$name.zip"; }
  (cd "$root" && go run -buildvcs=false ./tools/archive \
    -format "$format" -output "$archive" -root "$name" -epoch "$source_date_epoch" \
    -entry "$binary=$stage/$binary" \
    -entry "README.md=$stage/README.md" \
    -entry "LICENSE=$stage/LICENSE" \
    -entry "install.sh=$stage/install.sh" \
    -entry "uninstall.sh=$stage/uninstall.sh" \
    -entry "install.ps1=$stage/install.ps1" \
    -entry "uninstall.ps1=$stage/uninstall.ps1")
}

build_portable_archives() {
  for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
    archive_portable "${target%/*}" "${target#*/}"
  done
}

build_macos_pkg() {
  if ! command -v lipo >/dev/null 2>&1 || ! command -v pkgbuild >/dev/null 2>&1 || ! command -v productbuild >/dev/null 2>&1 || ! command -v pkgutil >/dev/null 2>&1 || ! command -v xar >/dev/null 2>&1; then
    echo "skipping macOS .pkg: lipo, pkgbuild, productbuild, pkgutil, or xar not available" >&2
    return 0
  fi
  pkg_root="$build_root/pkgroot"
  scripts_dir="$build_root/pkg-scripts"
  component="$build_root/aigw-component.pkg"
  universal="$build_root/aigw-universal"
  rm -rf "$pkg_root" "$scripts_dir"
  mkdir -p "$pkg_root/usr/local/bin" "$pkg_root/usr/local/libexec/aigw" "$scripts_dir"
  build_binary darwin amd64 pkg "$build_root/aigw-darwin-amd64"
  build_binary darwin arm64 pkg "$build_root/aigw-darwin-arm64"
  lipo -create -output "$universal" "$build_root/aigw-darwin-amd64" "$build_root/aigw-darwin-arm64"
  cp "$universal" "$pkg_root/usr/local/bin/aigw"
  chmod 755 "$pkg_root/usr/local/bin/aigw"
  cp "$root/packaging/macos/aigw-uninstall" "$pkg_root/usr/local/libexec/aigw/uninstall"
  chmod 755 "$pkg_root/usr/local/libexec/aigw/uninstall"
  cp "$root/packaging/macos/aigw-postinstall" "$scripts_dir/postinstall"
  chmod 755 "$scripts_dir/postinstall"
  normalize_tree_mtime "$pkg_root"
  normalize_tree_mtime "$scripts_dir"
  pkgbuild --root "$pkg_root" --scripts "$scripts_dir" --identifier "dig.aigw.cli" --version "$version" --install-location / "$component" >/dev/null
  product="$out_abs/aigw_${version}_darwin_universal.pkg"
  productbuild --package "$component" "$product" >/dev/null
  product_stage="$build_root/product-stage"
  canonical_product="$build_root/aigw-canonical.pkg"
  pkgutil --expand-full "$product" "$product_stage" >/dev/null
  (cd "$product_stage" && xar -c --distribution --compression=gzip -f "$canonical_product" Distribution aigw-component.pkg)
  (cd "$root" && go run -buildvcs=false ./tools/xarnorm -input "$canonical_product" -output "$product" -epoch "$source_date_epoch")
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

stable_uuid() {
  name=$1
  (cd "$root" && go run -buildvcs=false ./tools/releaseid \
    -namespace 6ba7b814-9dad-11d1-80b4-00c04fd430c8 \
    -name "$name")
}

normalize_tree_mtime() {
  tree=$1
  find "$tree" -exec touch -h -t "$source_date_touch" {} +
  xattr -cr "$tree"
  find "$tree" -type f -name '._*' -delete
}

write_msi_metadata_table() {
  artifact=$1
  arch=$2
  environment_guid=$3
  package_guid=$4
  environment_table="$build_root/msi-environment-$arch.idt"
  summary_table="$build_root/msi-summary-$arch.idt"
  msiinfo export "$artifact" _SummaryInformation > "$summary_table"
  python3 - "$environment_table" "$summary_table" "$environment_guid" "$package_guid" "$source_date_epoch" <<'PYTHON'
import datetime as dt
import sys
from pathlib import Path

environment_path = Path(sys.argv[1])
summary_path = Path(sys.argv[2])
environment_guid = sys.argv[3]
package_guid = sys.argv[4]
epoch = int(sys.argv[5])

def read_idt(path):
    return path.read_text(encoding="utf-8").splitlines()

def write_idt(path, lines):
    path.write_bytes(("\r\n".join(lines) + "\r\n").encode("utf-8"))

write_idt(
    environment_path,
    [
        "Environment\tName\tValue\tComponent_",
        "s72\tl64\tL255\ts72",
        "Environment\tEnvironment",
        f"{environment_guid}\t=PATH\t[~];[INSTALLBINFOLDER]\tAigwPath",
    ],
)

summary = read_idt(summary_path)
timestamp = dt.datetime.fromtimestamp(epoch, tz=dt.timezone.utc).strftime("%Y/%m/%d %H:%M:%S")
seen = set()
for index, line in enumerate(summary[3:], start=3):
    fields = line.split("\t")
    if len(fields) != 2:
        continue
    if fields[0] == "9":
        fields[1] = package_guid
        seen.add("9")
    elif fields[0] in {"12", "13"}:
        fields[1] = timestamp
        seen.add(fields[0])
    summary[index] = "\t".join(fields)
if seen != {"9", "12", "13"}:
    raise SystemExit("MSI summary information is missing deterministic fields")
write_idt(summary_path, summary)
PYTHON
  msibuild "$artifact" -i "$environment_table"
  msibuild "$artifact" -i "$summary_table"
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
    -e "s|\${AIGW_PACKAGE_MAINTAINER}|$(sed_escape "$package_maintainer")|g" \
    -e "s|\${AIGW_PACKAGE_HOMEPAGE}|$(sed_escape "$package_homepage")|g" \
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
Claude launcher is created later in AIGW's own data directory by the target user's aigw setup/repair command.
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

# shellcheck source=scripts/release/lib/msi-version.sh
. "$root/scripts/release/lib/msi-version.sh"

build_windows_msi() {
  if ! command -v wixl >/dev/null 2>&1; then
    echo "skipping Windows .msi: wixl not available" >&2
    return 0
  fi
  for arch in amd64 arm64; do
    wix_target=$(wix_arch "$arch")
    msi_stage="$build_root/msi-${arch}"
    rm -rf "$msi_stage"
    mkdir -p "$msi_stage"
    build_binary windows "$arch" msi "$msi_stage/aigw.exe"
    normalize_tree_mtime "$msi_stage"
    exe_guid=$(msi_component_guid "$arch" AigwExe)
    path_guid=$(msi_component_guid "$arch" AigwPath)
    product_guid=$(stable_uuid "aigw/product/$version/windows/$arch")
    package_guid=$(stable_uuid "aigw/package/$version/windows/$arch")
    environment_guid=$(stable_uuid "aigw/environment/$version/windows/$arch")
    if [ "$arch" = arm64 ] && ! wixl -a arm64 -E "$root/packaging/windows/aigw.wxs" >/dev/null 2>"$msi_stage/arm64-probe.err"; then
      wix_target=x64
    fi
    if wixl -a "$wix_target" -D "ProductVersion=$(msi_version)" -D "SourceDir=$msi_stage" \
      -D "AigwExeGuid=$exe_guid" -D "AigwPathGuid=$path_guid" -D "ProductCode=$product_guid" -D "PackageCode=$package_guid" \
      -o "$out_abs/aigw_${version}_windows_${arch}.msi" "$root/packaging/windows/aigw.wxs" >/dev/null 2>"$msi_stage/wixl.err"; then
      if [ "$arch" = arm64 ] && [ "$wix_target" = x64 ]; then
        if command -v msibuild >/dev/null 2>&1; then
          msibuild "$out_abs/aigw_${version}_windows_${arch}.msi" \
            -s "AIGW CLI" "AIGW CLI" "Arm64;1033" "$package_guid" >/dev/null
        else
          rm -f "$out_abs/aigw_${version}_windows_${arch}.msi"
          echo "skipping Windows arm64 .msi: wixl lacks arm64 and msibuild is unavailable; portable zip is still generated" >&2
          continue
        fi
      fi
      write_msi_metadata_table "$out_abs/aigw_${version}_windows_${arch}.msi" "$arch" "$environment_guid" "$package_guid"
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
(cd "$root" && go run -buildvcs=false ./tools/sbom -version "$version") > "$out_abs/aigw_${version}.spdx.json"
write_checksums
if [ "${AIGW_REQUIRE_FULL_MATRIX:-0}" = "1" ]; then
  "$root/scripts/checks/release/check-release-artifacts.sh" "$out_abs" "$version"
fi
printf 'release artifacts written to %s\n' "$out_abs"
