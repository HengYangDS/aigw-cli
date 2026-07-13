param(
    [Parameter(Mandatory = $true)]
    [string]$Installer
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $Installer)) { throw "installer does not exist: $Installer" }

$root = Join-Path ([IO.Path]::GetTempPath()) ("aigw-windows-installer-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $root | Out-Null
try {
    $testVersion = "0.0.0-win-fallback-test"
    $arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
    $testArchive = "aigw_${testVersion}_windows_${arch}.zip"
    $payloadRoot = Join-Path $root "payload"
    $payload = Join-Path $payloadRoot "aigw_${testVersion}_windows_${arch}"
    New-Item -ItemType Directory -Force -Path $payload | Out-Null
    Set-Content -NoNewline -Path (Join-Path $payload "aigw.exe") -Value "AIGW Windows fallback test payload"
    Compress-Archive -Path $payload -DestinationPath (Join-Path $root $testArchive)
    $hash = (Get-FileHash (Join-Path $root $testArchive) -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -NoNewline -Path (Join-Path $root "checksums.txt") -Value "$hash  $testArchive`n"

    $savedPath = $env:PATH
    $savedLocalAppData = $env:LOCALAPPDATA
    $savedInstallDir = $env:AIGW_INSTALL_DIR
    $savedHost = $env:AIGW_GL_HOST
    $savedVersion = $env:AIGW_VERSION
    $savedToken = $env:GITLAB_TOKEN
    $savedFixtureRoot = $env:AIGW_WINDOWS_INSTALLER_TEST_ROOT
    $savedFixtureVersion = $env:AIGW_WINDOWS_INSTALLER_TEST_VERSION
    $savedFixtureArchive = $env:AIGW_WINDOWS_INSTALLER_TEST_ARCHIVE
    $savedLatestCalls = $env:AIGW_WINDOWS_INSTALLER_TEST_LATEST_CALLS
    $savedDownloadCalls = $env:AIGW_WINDOWS_INSTALLER_TEST_DOWNLOAD_CALLS
    try {
        # No `glab` must be found: this forces the GitLab-token fallback.
        $env:PATH = "/usr/bin:/bin"
        $env:LOCALAPPDATA = Join-Path $root "localappdata"
        $env:AIGW_INSTALL_DIR = Join-Path $root "install"
        $env:AIGW_GL_HOST = "https://gitlab.invalid.test"
        $env:AIGW_VERSION = "latest"
        $env:GITLAB_TOKEN = "test-token"
        $env:AIGW_WINDOWS_INSTALLER_TEST_ROOT = $root
        $env:AIGW_WINDOWS_INSTALLER_TEST_VERSION = $testVersion
        $env:AIGW_WINDOWS_INSTALLER_TEST_ARCHIVE = $testArchive
        $env:AIGW_WINDOWS_INSTALLER_TEST_LATEST_CALLS = "0"
        $env:AIGW_WINDOWS_INSTALLER_TEST_DOWNLOAD_CALLS = "0"
        $script:latestCalls = 0
        $script:downloadCalls = 0
        function Invoke-RestMethod {
            param([string]$Uri, [hashtable]$Headers)
            if ($Uri -notmatch "/api/v4/projects/dig%2Fmisc%2Fagentic-third-party-api%2Faigw-cli/releases/permalink/latest$") { throw "unexpected latest URL: $Uri" }
            if ($Headers["PRIVATE-TOKEN"] -ne "test-token") { throw "latest request missing test token" }
            $env:AIGW_WINDOWS_INSTALLER_TEST_LATEST_CALLS = ([int]$env:AIGW_WINDOWS_INSTALLER_TEST_LATEST_CALLS + 1).ToString()
            [pscustomobject]@{ tag_name = "v$env:AIGW_WINDOWS_INSTALLER_TEST_VERSION" }
        }
        function Invoke-WebRequest {
            param([string]$Uri, [hashtable]$Headers, [string]$OutFile)
            if ($Headers["PRIVATE-TOKEN"] -ne "test-token") { throw "download request missing test token: $Uri" }
            $env:AIGW_WINDOWS_INSTALLER_TEST_DOWNLOAD_CALLS = ([int]$env:AIGW_WINDOWS_INSTALLER_TEST_DOWNLOAD_CALLS + 1).ToString()
            if ($Uri -match "/downloads/$([regex]::Escape($env:AIGW_WINDOWS_INSTALLER_TEST_ARCHIVE))$") {
                Copy-Item (Join-Path $env:AIGW_WINDOWS_INSTALLER_TEST_ROOT $env:AIGW_WINDOWS_INSTALLER_TEST_ARCHIVE) $OutFile -Force
            } elseif ($Uri -match "/downloads/checksums[.]txt$") {
                Copy-Item (Join-Path $env:AIGW_WINDOWS_INSTALLER_TEST_ROOT "checksums.txt") $OutFile -Force
            } else {
                throw "unexpected asset URL: $Uri"
            }
        }
        & $Installer
        $target = Join-Path $env:AIGW_INSTALL_DIR "aigw.exe"
        if (-not (Test-Path $target)) { throw "missing installed target: $target" }
        if ((Get-Content -Raw $target).Trim() -ne "AIGW Windows fallback test payload") { throw "installed payload mismatch" }
        if ($env:AIGW_WINDOWS_INSTALLER_TEST_LATEST_CALLS -ne "1" -or $env:AIGW_WINDOWS_INSTALLER_TEST_DOWNLOAD_CALLS -ne "2") { throw "unexpected mocked request counts: latest=$env:AIGW_WINDOWS_INSTALLER_TEST_LATEST_CALLS downloads=$env:AIGW_WINDOWS_INSTALLER_TEST_DOWNLOAD_CALLS" }

        # Reject header/control-character injection before even a local-source
        # installation branch can run.
        $controlDirectory = Join-Path $root "control-character"
        New-Item -ItemType Directory -Path $controlDirectory | Out-Null
        Copy-Item $Installer (Join-Path $controlDirectory "install.ps1")
        Set-Content -NoNewline -Path (Join-Path $controlDirectory "aigw.exe") -Value "must-not-install"
        $env:GITLAB_TOKEN = "test-token`nInjected: no"
        $rejected = $false
        try {
            & (Join-Path $controlDirectory "install.ps1")
        } catch {
            if ($_.Exception.Message -like "*GITLAB_TOKEN contains a control character*") { $rejected = $true }
            else { throw }
        }
        if (-not $rejected) { throw "installer accepted a control-character-bearing GITLAB_TOKEN" }
    } finally {
        $env:PATH = $savedPath
        $env:LOCALAPPDATA = $savedLocalAppData
        $env:AIGW_INSTALL_DIR = $savedInstallDir
        $env:AIGW_GL_HOST = $savedHost
        $env:AIGW_VERSION = $savedVersion
        $env:GITLAB_TOKEN = $savedToken
        $env:AIGW_WINDOWS_INSTALLER_TEST_ROOT = $savedFixtureRoot
        $env:AIGW_WINDOWS_INSTALLER_TEST_VERSION = $savedFixtureVersion
        $env:AIGW_WINDOWS_INSTALLER_TEST_ARCHIVE = $savedFixtureArchive
        $env:AIGW_WINDOWS_INSTALLER_TEST_LATEST_CALLS = $savedLatestCalls
        $env:AIGW_WINDOWS_INSTALLER_TEST_DOWNLOAD_CALLS = $savedDownloadCalls
    }
    Write-Output "PowerShell GitLab-token latest fallback, standard SHA-256 manifest, and install: OK"
} finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $root
}
