param([string]$InstallDir = $(if ($env:AIGW_INSTALL_DIR) { $env:AIGW_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\aigw" }))
$ErrorActionPreference = "Stop"
$shim = Join-Path $InstallDir "claude.cmd"
if (Test-Path $shim) {
    if ((Get-Content $shim -Raw) -notmatch "AIGW managed Claude shim") { throw "Refusing to remove non-AIGW Claude launcher: $shim" }
    Remove-Item $shim -Force
}
Remove-Item (Join-Path $InstallDir "aigw.exe") -Force -ErrorAction SilentlyContinue
Write-Host "Removed AIGW executable and owned launcher. Configuration and Credential Manager secrets were preserved."
