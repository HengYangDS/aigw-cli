param(
    [string]$InstallDir,
    [switch]$NoPath,
    [switch]$Help
)

$ErrorActionPreference = "Stop"
if ($Help) {
    Write-Output "Usage: install.ps1 [-InstallDir <path>] [-NoPath] [-Help]"
    Write-Output "Install the bundled AIGW binary for the current user."
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

$LocalBinary = Join-Path $PSScriptRoot "aigw.exe"
if (-not (Test-Path $LocalBinary)) {
    throw "This installer only installs the bundled aigw.exe. Download and extract the matching portable zip first; use aigw update after installation."
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$target = Join-Path $InstallDir "aigw.exe"
$backup = Join-Path $InstallDir ".aigw.previous.exe"
if (Test-Path $target) {
    Copy-Item $target "$backup.new" -Force
    Move-Item "$backup.new" $backup -Force
}
Copy-Item $LocalBinary "$target.new" -Force
Move-Item "$target.new" $target -Force

if (-not $NoPath) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $parts = @()
    if ($userPath) { $parts = $userPath -split ';' | Where-Object { $_ } }
    if ($parts -notcontains $InstallDir) {
        $newPath = (($parts + $InstallDir) -join ';')
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Host "User PATH updated. Open a new terminal to use aigw."
    }
}
Write-Host "Installed $target"
if (Test-Path $backup) { Write-Host "Previous AIGW binary saved to $backup" }
Write-Host "Next: aigw setup"
