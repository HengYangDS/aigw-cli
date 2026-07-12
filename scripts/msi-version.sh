#!/bin/sh

# MSI ProductVersion is a three-field integer tuple. Preserve SemVer ordering
# by encoding <patch, prerelease stage> into its third field:
#   patch * 256 + stage
# stage: unknown prerelease=0, alpha.1..63, beta.1..64, rc.1..127,
# stable=255. This supports patch values 0..255 and guarantees that every
# prerelease sorts below the matching stable release and above the prior patch.
msi_version() {
  clean=${version#v}
  core=${clean%%-*}
  old_ifs=$IFS
  IFS=.
  set -- $core
  IFS=$old_ifs
  major=${1:-}
  minor=${2:-}
  patch=${3:-}
  [ -n "$major" ] && [ -n "$minor" ] && [ -n "$patch" ] && [ "$#" -eq 3 ] || {
    echo "invalid MSI version core: $core" >&2
    return 2
  }
  case "$major:$minor:$patch" in
    *[!0-9:]*|:*|*::*) echo "invalid MSI version core: $core" >&2; return 2 ;;
  esac
  [ "$major" -le 255 ] 2>/dev/null && [ "$minor" -le 255 ] 2>/dev/null && [ "$patch" -le 255 ] 2>/dev/null || {
    echo "MSI version component out of range: $core" >&2
    return 2
  }

  stage=255
  case "$clean" in
    *-rc.*)
      sequence=${clean##*-rc.}
      case "$sequence" in ''|*[!0-9]*) echo "invalid RC version: $clean" >&2; return 2 ;; esac
      [ "$sequence" -ge 1 ] 2>/dev/null && [ "$sequence" -le 127 ] 2>/dev/null || {
        echo "RC sequence out of range: $clean" >&2
        return 2
      }
      stage=$((127 + sequence))
      ;;
    *-beta.*)
      sequence=${clean##*-beta.}
      case "$sequence" in ''|*[!0-9]*) echo "invalid beta version: $clean" >&2; return 2 ;; esac
      [ "$sequence" -ge 1 ] 2>/dev/null && [ "$sequence" -le 64 ] 2>/dev/null || {
        echo "beta sequence out of range: $clean" >&2
        return 2
      }
      stage=$((63 + sequence))
      ;;
    *-alpha.*)
      sequence=${clean##*-alpha.}
      case "$sequence" in ''|*[!0-9]*) echo "invalid alpha version: $clean" >&2; return 2 ;; esac
      [ "$sequence" -ge 1 ] 2>/dev/null && [ "$sequence" -le 63 ] 2>/dev/null || {
        echo "alpha sequence out of range: $clean" >&2
        return 2
      }
      stage=$sequence
      ;;
    *-*) stage=0 ;;
  esac

  build=$((patch * 256 + stage))
  printf '%s.%s.%s' "$major" "$minor" "$build"
}
