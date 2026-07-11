#!/bin/sh
set -eu

version=${1:?usage: check-release-readiness.sh <version>}

case "$version" in
  *-rc.*|*-beta.*|*-alpha.*)
    echo "release readiness: $version is a non-GA build; checksums and SBOM are required, signing is not claimed"
    exit 0
    ;;
esac

cat >&2 <<'MSG'
GA release blocked: this repository does not yet contain the protected
organization signing and notarization jobs required for production assets.

Before publishing a GA tag, add and verify all of the following in protected CI:
  - macOS Developer ID binary/package signing and notarization/stapling
  - Windows Authenticode signing on a managed Windows runner
  - artifact signature verification before publish/release

Do not bypass this gate with an environment variable or unsigned manual upload.
MSG
exit 1
