param([string]$InstallDir = $(if ($env:AIGW_INSTALL_DIR) { $env:AIGW_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\aigw\bin" }))

$ErrorActionPreference = "Stop"
$LocalBinary = Join-Path $PSScriptRoot "aigw.exe"
if (-not (Test-Path $LocalBinary)) {
    throw "This installer only installs the bundled aigw.exe. Download and extract the matching portable zip first; use aigw update after installation."
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$target = Join-Path $InstallDir "aigw.exe"
Copy-Item $LocalBinary "$target.new" -Force
Move-Item "$target.new" $target -Force

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$parts = @()
if ($userPath) { $parts = $userPath -split ';' | Where-Object { $_ } }
if ($parts -notcontains $InstallDir) {
    $newPath = (($parts + $InstallDir) -join ';')
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "User PATH updated. Open a new terminal to use aigw."
}
Write-Host "Installed $target"
Write-Host "Next: aigw setup"
