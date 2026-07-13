param(
    [Parameter(Mandatory = $true)]
    [string]$Installer
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path $Installer)) { throw "installer does not exist: $Installer" }
$text = Get-Content -Raw $Installer
foreach ($forbidden in @("Invoke-WebRequest", "Invoke-RestMethod", "GITLAB_TOKEN", "AIGW_GL_HOST", "glab")) {
    if ($text -match [regex]::Escape($forbidden)) { throw "portable installer must not implement network release retrieval: $forbidden" }
}

$root = Join-Path ([IO.Path]::GetTempPath()) ("aigw-windows-installer-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $root | Out-Null
try {
    $savedLocalAppData = $env:LOCALAPPDATA
    $savedInstallDir = $env:AIGW_INSTALL_DIR
    $savedUserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    try {
        $package = Join-Path $root "package"
        New-Item -ItemType Directory -Path $package | Out-Null
        Copy-Item $Installer (Join-Path $package "install.ps1")
        Set-Content -NoNewline -Path (Join-Path $package "aigw.exe") -Value "AIGW Windows bundled installer payload"
        $env:LOCALAPPDATA = Join-Path $root "localappdata"
        $env:AIGW_INSTALL_DIR = Join-Path $root "install"
        & (Join-Path $package "install.ps1")
        $target = Join-Path $env:AIGW_INSTALL_DIR "aigw.exe"
        if (-not (Test-Path $target)) { throw "missing installed target" }
        if ((Get-Content -Raw $target).Trim() -ne "AIGW Windows bundled installer payload") { throw "installed payload mismatch" }

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
