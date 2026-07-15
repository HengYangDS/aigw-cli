param(
    [Parameter(Mandatory = $true)]
    [string]$Installer
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path $Installer)) { throw "installer does not exist: $Installer" }
$Uninstaller = Join-Path (Split-Path -Parent $Installer) "uninstall.ps1"
if (-not (Test-Path $Uninstaller)) { throw "uninstaller does not exist beside installer: $Uninstaller" }
$text = Get-Content -Raw $Installer
foreach ($forbidden in @("Invoke-WebRequest", "Invoke-RestMethod", "curl", "wget", "gh", "glab", "GITHUB_TOKEN", "GITLAB_TOKEN", "AIGW_GITLAB_RELEASE_HOST", "AIGW_GITLAB_RELEASE_PROJECT", "AIGW_GITHUB_RELEASE_HOST", "AIGW_GITHUB_RELEASE_PROJECT")) {
    if ($text -match [regex]::Escape($forbidden)) { throw "portable installer must not implement network release retrieval: $forbidden" }
}

$root = Join-Path ([IO.Path]::GetTempPath()) ("aigw-windows-installer-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $root | Out-Null
try {
    $savedLocalAppData = $env:LOCALAPPDATA
    $savedInstallDir = $env:AIGW_INSTALL_DIR
    $savedUserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    try {
        $helpPackage = Join-Path $root "help-package"
        New-Item -ItemType Directory -Path $helpPackage | Out-Null
        Copy-Item $Installer (Join-Path $helpPackage "install.ps1")
        Copy-Item $Uninstaller (Join-Path $helpPackage "uninstall.ps1")
        $env:LOCALAPPDATA = $null
        $env:AIGW_INSTALL_DIR = $null
        $helpText = (& (Join-Path $helpPackage "install.ps1") -Help | Out-String)
        if ($helpText -notmatch [regex]::Escape("Usage: install.ps1")) { throw "installer help did not print usage without a Windows environment" }
        $uninstallHelpText = (& (Join-Path $helpPackage "uninstall.ps1") -Help | Out-String)
        if ($uninstallHelpText -notmatch [regex]::Escape("Usage: uninstall.ps1")) { throw "uninstaller help did not print usage without a Windows environment" }

        $package = Join-Path $root "package"
        New-Item -ItemType Directory -Path $package | Out-Null
        Copy-Item $Installer (Join-Path $package "install.ps1")
        Copy-Item $Uninstaller (Join-Path $package "uninstall.ps1")
        Set-Content -NoNewline -Path (Join-Path $package "aigw.exe") -Value "AIGW Windows bundled installer payload"
        $env:LOCALAPPDATA = Join-Path $root "localappdata"
        $env:AIGW_INSTALL_DIR = Join-Path $root "install"
        & (Join-Path $package "install.ps1") -Help | Out-Null
        if (Test-Path (Join-Path $env:AIGW_INSTALL_DIR "aigw.exe")) { throw "installer help modified the installation" }

        New-Item -ItemType Directory -Force -Path $env:AIGW_INSTALL_DIR | Out-Null
        $target = Join-Path $env:AIGW_INSTALL_DIR "aigw.exe"
        Set-Content -NoNewline -Path $target -Value "AIGW Windows previous payload"
        & (Join-Path $package "install.ps1")
        if (-not (Test-Path $target)) { throw "missing installed target" }
        if ((Get-Content -Raw $target).Trim() -ne "AIGW Windows bundled installer payload") { throw "installed payload mismatch" }
        $backup = Join-Path $env:AIGW_INSTALL_DIR ".aigw.previous.exe"
        if (-not (Test-Path $backup)) { throw "installer did not retain the immediately preceding AIGW binary" }
        if ((Get-Content -Raw $backup).Trim() -ne "AIGW Windows previous payload") { throw "installer backup payload mismatch" }

        & (Join-Path $package "uninstall.ps1") -Help | Out-Null
        if (-not (Test-Path $target)) { throw "uninstaller help removed the AIGW binary" }
        & (Join-Path $package "uninstall.ps1")
        if (Test-Path $target) { throw "uninstaller left the AIGW binary" }
        if (Test-Path $backup) { throw "uninstaller left the AIGW rollback binary" }

        $missing = Join-Path $root "missing"
        New-Item -ItemType Directory -Path $missing | Out-Null
        Copy-Item $Installer (Join-Path $missing "install.ps1")
        $env:AIGW_INSTALL_DIR = Join-Path $root "missing-install"
        $rejected = $false
        try { & (Join-Path $missing "install.ps1") } catch {
            if ($_.Exception.Message -like "*only installs the bundled aigw.exe*") { $rejected = $true } else { throw }
        }
        if (-not $rejected) { throw "installer accepted a missing bundled aigw.exe" }
    } finally {
        $env:LOCALAPPDATA = $savedLocalAppData
        $env:AIGW_INSTALL_DIR = $savedInstallDir
        [Environment]::SetEnvironmentVariable("Path", $savedUserPath, "User")
    }
    Write-Output "PowerShell bundled portable installer contract: OK"
} finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $root
}
