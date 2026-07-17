#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

cp "$root/scripts/check-text-layout.py" "$tmp/check.py"
mkdir -p "$tmp/repository"
cd "$tmp/repository"
git init -q
git config user.name Test
git config user.email test@example.invalid
mkdir -p docs
printf '# Valid\n\nOne paragraph.\n' > README.md
git add README.md
git commit -qm initial
# The checker discovers its repository root from its own path; keep this a
# black-box contract test by mirroring the production location.
mkdir -p scripts
mv "$tmp/check.py" scripts/check-text-layout.py
python3 scripts/check-text-layout.py >/dev/null
# macOS includes a system Python that may be older than a developer-managed
# interpreter. The checker is repository tooling, so it must remain runnable
# without Homebrew or another runtime manager. Other platforms may inject an
# equivalent baseline interpreter through AIGW_TEXT_LAYOUT_COMPAT_PYTHON.
compat_python=${AIGW_TEXT_LAYOUT_COMPAT_PYTHON:-}
if [ -z "$compat_python" ] && [ "$(uname -s)" = Darwin ] && [ -x /usr/bin/python3 ]; then
  compat_python=/usr/bin/python3
fi
if [ -n "$compat_python" ]; then
  "$compat_python" scripts/check-text-layout.py >/dev/null
fi

# The checker must remain independent of a caller's fsmonitor setting. Route
# its Git subprocess through a contract wrapper so the invocation itself is
# checked, while the real Git process still validates the result.
real_git=$(command -v git)
git_wrapper=$tmp/git-wrapper
mkdir -p "$git_wrapper"
cat > "$git_wrapper/git" <<'SH'
#!/bin/sh
if [ "$1" != "-c" ] || [ "$2" != "core.fsmonitor=false" ] || [ "$3" != "ls-files" ] || [ "$4" != "-z" ]; then
  echo 'text layout checker did not disable fsmonitor for tracked-file discovery' >&2
  exit 1
fi
exec "$AIGW_TEXT_LAYOUT_REAL_GIT" "$@"
SH
chmod +x "$git_wrapper/git"
env \
  AIGW_TEXT_LAYOUT_REAL_GIT="$real_git" \
  GIT_CONFIG_COUNT=1 \
  GIT_CONFIG_KEY_0=core.fsmonitor \
  GIT_CONFIG_VALUE_0=true \
  PATH="$git_wrapper:$PATH" \
  python3 scripts/check-text-layout.py >/dev/null
printf '# Invalid\n\n\nDouble gap.\n' > README.md
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted consecutive Markdown blank lines' >&2
  exit 1
fi
printf '# Valid\n\nOne paragraph.\n' > README.md
printf 'def one():\n    return 1\n\n\ndef two():\n    return 2\n' > docs/style.py
git add docs/style.py
python3 scripts/check-text-layout.py >/dev/null
printf 'def one():\n    return 1\n\ndef two():\n    return 2\n' > docs/style.py
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a missing Python module separator' >&2
  exit 1
fi
printf 'class Example:\n    def one(self):\n        return 1\n\n    def two(self):\n        return 2\n' > docs/style.py
if ! python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker rejected a single Python class-method separator' >&2
  exit 1
fi
printf 'class Example:\n    def one(self):\n        return 1\n\n\n\n    def two(self):\n        return 2\n' > docs/style.py
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted an extra Python class separator' >&2
  exit 1
fi
printf 'class Example:\n    def one(self):\n        return 1\n\n    def two(self):\n        return 2\n' > docs/style.py
printf 'def compact():\n    value = 1\n\n\n    return value\n' > docs/style.py
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a double blank line inside a Python function' >&2
  exit 1
fi
printf 'def compact():\n    value = 1\n\n    return value\n' > docs/style.py
printf 'def documented():\n    value = """First paragraph.\n\n\nSecond paragraph."""\n    return value\n' > docs/style.py
if ! python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker rejected literal whitespace inside a Python multiline string' >&2
  exit 1
fi
printf 'root = true\n\n\n[*.go]\nindent_style = tab\n' > .editorconfig
git add .editorconfig
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a double configuration separator' >&2
  exit 1
fi
printf 'root = true\n\n[*.go]\nindent_style = tab\n' > .editorconfig
python3 scripts/check-text-layout.py >/dev/null
printf 'module example\n\n\ngo 1.24.0\n' > go.mod
git add go.mod
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a double Go module separator' >&2
  exit 1
fi
printf 'module example\n\ngo 1.24.0\n' > go.mod
python3 scripts/check-text-layout.py >/dev/null
printf 'version = 2\n\n# The comment belongs to the table that follows.\n[profile]\nmodel = "example"\n' > config.toml
git add config.toml
python3 scripts/check-text-layout.py >/dev/null
printf 'version = 2\n[profile]\nmodel = "example"\n' > config.toml
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a missing TOML table separator' >&2
  exit 1
fi
printf 'version = 2\n# The comment belongs to the table that follows.\n[profile]\nmodel = "example"\n' > config.toml
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a missing TOML comment-and-table separator' >&2
  exit 1
fi
printf 'version = 2\n\n# The comment belongs to the table that follows.\n[profile]\nmodel = "example"\n' > config.toml
python3 scripts/check-text-layout.py >/dev/null
printf '#!/bin/sh\n\nprintf %s ready\n' > aigw-postinstall
git add aigw-postinstall
python3 scripts/check-text-layout.py >/dev/null
printf '#!/bin/sh\n\n\n\nprintf %s ready\n' > aigw-postinstall
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a double extensionless-script separator' >&2
  exit 1
fi
printf '#!/bin/sh\n\nprintf %s ready\n' > aigw-postinstall
python3 scripts/check-text-layout.py >/dev/null
printf '{\n\n\n  "name": "example"\n}\n' > config.json
git add config.json
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a double JSON configuration separator' >&2
  exit 1
fi
printf '{\n\n  "name": "example"\n}\n' > config.json
python3 scripts/check-text-layout.py >/dev/null
printf '\0\1\2' > fixture.bin
git add fixture.bin
python3 scripts/check-text-layout.py >/dev/null
printf '# Invalid\nText with trailing space \n' > README.md
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted trailing whitespace' >&2
  exit 1
fi
printf '# Invalid\n\n' > README.md
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a terminal blank line' >&2
  exit 1
fi
printf '# Valid\n\n```text\n\n\nLiteral gap retained.\n```\n' > README.md
printf 'class Example:\n    def one(self):\n        return 1\n\n    def two(self):\n        return 2\n' > docs/style.py
printf 'root = true\n\n[*.go]\nindent_style = tab\n' > .editorconfig
printf 'module example\n\ngo 1.24.0\n' > go.mod
printf 'version = 2\n\n[profile]\nmodel = "example"\n' > config.toml
printf '#!/bin/sh\n\nprintf %s ready\n' > aigw-postinstall
printf '{\n\n  "name": "example"\n}\n' > config.json
python3 scripts/check-text-layout.py >/dev/null

echo 'Text layout contract test: OK'
