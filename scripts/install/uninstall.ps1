param(
    [string]$InstallDir,
    [switch]$Help
)
$ErrorActionPreference = "Stop"
if ($Help) {
    Write-Output "Usage: uninstall.ps1 [-InstallDir <path>] [-Help]"
    Write-Output "Remove only files and PATH entries owned by AIGW."
    return
}
if (-not $InstallDir) {
    if ($env:AIGW_INSTALL_DIR) {
        $InstallDir = $env:AIGW_INSTALL_DIR
    } elseif ($env:LOCALAPPDATA) {
        $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\aigw\bin"
    } else {
        throw "InstallDir is required when LOCALAPPDATA is unavailable."
    }
}

$launcher = Join-Path $InstallDir "claude.cmd"
if (Test-Path $launcher) {
    if ((Get-Content $launcher -Raw) -notmatch "AIGW managed Claude launcher") { throw "Refusing to remove non-AIGW Claude launcher: $launcher" }
    Remove-Item $launcher -Force
}
Remove-Item (Join-Path $InstallDir "aigw.exe") -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path $InstallDir ".aigw.previous.exe") -Force -ErrorAction SilentlyContinue
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath) {
    $parts = $userPath -split ';' | Where-Object { $_ -and ($_ -ne $InstallDir) }
    [Environment]::SetEnvironmentVariable("Path", ($parts -join ';'), "User")
}
Write-Host "Removed AIGW executable, owned launcher, and AIGW PATH entry. Configuration and Credential Manager secrets were preserved."
