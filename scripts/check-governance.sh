#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

require_file() {
  test -f "$1" || { echo "missing governance document: $1" >&2; exit 1; }
}

for file in \
  AGENTS.md \
  LICENSE \
  CONTRIBUTING.md \
  docs/README.md \
  docs/architecture/authority-and-projection-boundary.md \
  docs/governance/change-and-release-policy.md \
  docs/governance/terminal-experience-contract.md \
  docs/decisions/0001-control-plane-data-plane-boundary.md \
  docs/evidence/README.md \
  .config/checks/coverage/policy.toml \
  packaging/release/gitlab-allowed-signers \
  packaging/release/github-allowed-signers \
  packaging/release/github-legacy-allowed-signers \
  packaging/release/github-legacy-tags.txt \
  packaging/release/retired-gitlab-tags.txt \
  packaging/release/verified-commit-floors.txt \
  .github/workflows/verify.yml \
  scripts/check-branch-closeout.sh \
  scripts/check-forge-sync.sh \
  scripts/check-static-analysis.sh \
  scripts/check-commit-provenance.sh \
  scripts/check-tag-namespace.sh \
  scripts/compare-ordered-trees.py \
  scripts/test-forge-sync.sh \
  scripts/test-commit-provenance.sh
do
  require_file "$file"
done

for gate in \
  'go run ./tools/coveragegate --race' \
  'go vet ./...' \
  'sh scripts/check-static-analysis.sh' \
  'test -z "$(gofmt -l cmd internal tools)"' \
  'sh scripts/check-governance.sh' \
  'sh scripts/check-commit-provenance.sh . gitlab' \
  'sh scripts/test-commit-provenance.sh' \
  'sh scripts/check-tag-namespace.sh' \
  'python3 scripts/check-markdown-presentation.py' \
  'python3 scripts/check-text-layout.py' \
  'sh scripts/test-text-layout.sh' \
  'sh scripts/test-changelog.sh'
do
  for document in CONTRIBUTING.md AGENTS.md README.md; do
    if ! grep -Fxq "$gate" "$document"; then
      echo "$document must list required local verification gate exactly: $gate" >&2
      exit 1
    fi
  done
done

for document in CONTRIBUTING.md AGENTS.md README.md; do
  if grep -Fq 'go test -race ./...' "$document"; then
    echo "$document bypasses the required coverage gate" >&2
    exit 1
  fi
done

sh scripts/check-changelog.sh
sh scripts/check-tag-namespace.sh
sh scripts/check-english-text.sh
sh scripts/check-product-surface.sh
python3 scripts/check-text-layout.py

if ! grep -Fq '# AIGW CLI' README.md; then
  echo "README.md must use the formal Project Name as its title" >&2
  exit 1
fi
if ! grep -Fq '`aigw-cli`' README.md; then
  echo "README.md must declare the stable GitLab Path separately" >&2
  exit 1
fi
if ! grep -Fq 'sh scripts/check-governance.sh' .gitlab-ci.yml; then
  echo "GitLab CI must execute the governance check" >&2
  exit 1
fi
if ! grep -Fq 'scripts/check-static-analysis.sh' .github/workflows/verify.yml; then
  echo "GitHub Actions must execute the static-analysis check" >&2
  exit 1
fi
if ! grep -Fq 'scripts/check-static-analysis.sh' .gitlab-ci.yml; then
  echo "GitLab CI must execute the static-analysis check" >&2
  exit 1
fi
if ! grep -Fq 'go tool staticcheck' scripts/check-static-analysis.sh || ! grep -Fq 'go tool errcheck ./...' scripts/check-static-analysis.sh; then
  echo "static-analysis check must run the tracked Staticcheck and Errcheck tools" >&2
  exit 1
fi
if ! grep -Fq 'scripts/check-governance.sh' .github/workflows/verify.yml; then
  echo "GitHub Actions must execute the governance check" >&2
  exit 1
fi
if test -e docs/history || test -e docs/superpowers || test -e docs/design || test -e docs/reviews || test -e docs/specs; then
  echo "retired documentary paths must not remain in the canonical tree" >&2
  exit 1
fi
if ! grep -Fxq '.serena/' .gitignore; then
  echo ".gitignore must exclude local Serena project metadata" >&2
  exit 1
fi

# AIGW CLI is an English-only repository.  Use explicit Unicode ranges instead
# of a grep Unicode-property dialect so this gate behaves the same on macOS and
# Linux runners.  Test fixtures are included deliberately: they are part of the
# maintainable project surface, not an exemption for stale product copy.
python3 - <<'PY'
from pathlib import Path
import subprocess
import sys

def is_serena_metadata(name: str) -> bool:
    return ".serena" in name.split("/")

for fixture in [".serena", ".serena/project.yml", "docs/.serena/project.yml"]:
    if not is_serena_metadata(fixture):
        raise SystemExit(f"Serena metadata matcher missed fixture: {fixture}")
for fixture in ["serena/project.yml", ".serenade/project.yml", "docs/serena/project.yml"]:
    if is_serena_metadata(fixture):
        raise SystemExit(f"Serena metadata matcher rejected safe fixture: {fixture}")

tracked = subprocess.check_output(["git", "ls-files", "-z"]).decode().split("\0")
serena_matches = [name for name in tracked if name and is_serena_metadata(name)]
if serena_matches:
    print("\n".join(serena_matches), file=sys.stderr)
    raise SystemExit("local Serena project metadata must not be tracked")

han = set(range(0x3400, 0x4DC0)) | set(range(0x4E00, 0xA000)) | set(range(0xF900, 0xFB00))
matches = []
for name in tracked:
    if not name or name == "scripts/check-english-text.sh":
        continue
    path = Path(name)
    try:
        text = path.read_text(encoding="utf-8")
    except (UnicodeDecodeError, OSError):
        continue
    for line_number, line in enumerate(text.splitlines(), 1):
        if any(ord(character) in han for character in line):
            matches.append(f"{name}:{line_number}:{line}")
if matches:
    print("\n".join(matches), file=sys.stderr)
    raise SystemExit("AIGW CLI repository content must be English-only")
PY
