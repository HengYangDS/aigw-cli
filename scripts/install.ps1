param(
    [string]$InstallDir = $(if ($env:AIGW_INSTALL_DIR) { $env:AIGW_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\aigw" }),
    [string]$Version = $(if ($env:AIGW_VERSION) { $env:AIGW_VERSION } else { "latest" })
)
$ErrorActionPreference = "Stop"
$Project = "dig/misc/agentic-third-party-api/aigw-cli"
$HostURL = if ($env:AIGW_GL_HOST) { $env:AIGW_GL_HOST } else { "http://192.168.64.101:18086" }
$LocalBinary = Join-Path $PSScriptRoot "aigw.exe"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

if (Test-Path $LocalBinary) {
    $Source = $LocalBinary
} else {
    if ($Version -eq "latest") {
        if (-not (Get-Command glab -ErrorAction SilentlyContinue)) { throw "glab is required to resolve the latest private release; set AIGW_VERSION or install glab" }
        $release = glab release list -R $Project --per-page 1 --format json | ConvertFrom-Json
        $Version = $release[0].tag_name
    }
    $clean = $Version.TrimStart("v")
	$tag = if ($Version.StartsWith("v")) { $Version } else { "v$Version" }
    $arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
    $archive = "aigw_${clean}_windows_${arch}.zip"
    $temp = Join-Path ([IO.Path]::GetTempPath()) ("aigw-" + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $temp | Out-Null
    if (Get-Command glab -ErrorAction SilentlyContinue) {
        $env:GL_HOST = $HostURL
        glab release download $tag -R $Project --asset-name $archive --dir $temp
        if ($LASTEXITCODE -ne 0) { throw "glab release download failed" }
        glab release download $tag -R $Project --asset-name checksums.txt --dir $temp
        if ($LASTEXITCODE -ne 0) { throw "glab checksum download failed" }
    } elseif ($env:GITLAB_TOKEN) {
        $url = "$HostURL/$Project/-/releases/$tag/downloads/$archive"
        Invoke-WebRequest -Uri $url -Headers @{"PRIVATE-TOKEN" = $env:GITLAB_TOKEN} -OutFile (Join-Path $temp $archive)
        Invoke-WebRequest -Uri "$HostURL/$Project/-/releases/$tag/downloads/checksums.txt" -Headers @{"PRIVATE-TOKEN" = $env:GITLAB_TOKEN} -OutFile (Join-Path $temp "checksums.txt")
    } else {
        throw "private release download requires authenticated glab or GITLAB_TOKEN"
    }
    $line = Get-Content (Join-Path $temp "checksums.txt") | Where-Object { $_ -match "(^|[\\/])$([regex]::Escape($archive))$" } | Select-Object -First 1
    if (-not $line) { throw "checksum entry missing for $archive" }
    $expected = ($line -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash (Join-Path $temp $archive) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "SHA-256 mismatch for $archive" }
    Expand-Archive (Join-Path $temp $archive) -DestinationPath $temp
    $Source = Join-Path $temp "aigw_${clean}_windows_${arch}\aigw.exe"
}

$target = Join-Path $InstallDir "aigw.exe"
Copy-Item $Source "$target.new" -Force
Move-Item "$target.new" $target -Force
Write-Host "Installed $target"
if (($env:PATH -split ';') -notcontains $InstallDir) { Write-Host "Add $InstallDir to your user PATH, then run: aigw setup" }
