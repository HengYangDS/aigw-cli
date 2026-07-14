param(
    [Parameter(Mandatory = $true)]
    [string]$Repository
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if (-not $IsWindows) { throw "native Windows acceptance must run on Windows" }
if (-not (Test-Path (Join-Path $Repository "go.mod"))) { throw "repository does not contain go.mod: $Repository" }

$originalLocation = Get-Location
$root = Join-Path ([IO.Path]::GetTempPath()) ("aigw-windows-native-" + [guid]::NewGuid())
try {
    Set-Location $Repository

    & go test -race ./...
    if ($LASTEXITCODE -ne 0) { throw "go test -race ./... failed" }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet ./... failed" }

    $binary = Join-Path $root "aigw.exe"
    New-Item -ItemType Directory -Force -Path $root | Out-Null
    & go build -o $binary ./cmd/aigw
    if ($LASTEXITCODE -ne 0) { throw "build Windows AIGW executable failed" }
    $version = & $binary --version
    if ($LASTEXITCODE -ne 0 -or $version -notmatch '^aigw version ') { throw "aigw.exe --version failed: $version" }

    $legacy = & powershell.exe -NoProfile -Command "`$env:NO_COLOR=''; & '$binary' --help" | Out-String
    if ($legacy -match "`e\[") { throw "legacy ConsoleHost emitted raw ANSI control sequences" }

    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Repository "scripts/test-installers.ps1") -Installer (Join-Path $Repository "scripts/install.ps1")
    if ($LASTEXITCODE -ne 0) { throw "portable Windows installer harness failed" }

    & go test ./internal/shims -run '^TestManagerCreatesWindowsCommandShimThatCanRunAIGW$' -count=1
    if ($LASTEXITCODE -ne 0) { throw "Windows Claude shim execution contract failed" }

    Write-Output "Windows native acceptance: OK"
} finally {
    Set-Location $originalLocation
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $root
}
