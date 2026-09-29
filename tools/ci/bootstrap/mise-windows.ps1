param(
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$Sha256,
    [Parameter(Mandatory = $true)][string]$Directory,
    [Parameter(Mandatory = $true)][string]$MirrorResource,
    [Parameter(Mandatory = $true)][string]$ReleaseMetadataPattern,
    [Parameter(Mandatory = $true)][string]$ReleaseMetadataResource
)

if ($Version -notmatch '^\d{4}\.\d+\.\d+$' -or $Sha256 -notmatch '^[0-9a-fA-F]{64}$' -or
    $MirrorResource -notmatch '^packages/generic/[a-z0-9-]+/v[1-9][0-9]*/$') {
    throw 'Invalid pinned Mise version, SHA-256, or mirror resource.'
}
foreach ($name in @('CI_API_V4_URL', 'CI_PROJECT_ID', 'CI_PROJECT_DIR', 'CI_JOB_ID', 'CI_JOB_TOKEN', 'CI_SERVER_HOST')) {
    if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name))) {
        throw "Missing GitLab job input: $name"
    }
}

$expected = Join-Path (Join-Path $env:CI_PROJECT_DIR 'build/tmp') "ci-mise-$env:CI_JOB_ID"
if ([IO.Path]::GetFullPath($Directory) -ne [IO.Path]::GetFullPath($expected)) {
    throw 'Mise bootstrap directory is not owned by this job.'
}
if (Test-Path -LiteralPath $Directory) {
    throw 'Mise bootstrap directory already exists.'
}

$name = "mise-v$Version-windows-arm64.zip"
$uri = "$env:CI_API_V4_URL/projects/$env:CI_PROJECT_ID/packages/generic/ci-mise/$Version/$name"
$archive = Join-Path $Directory $name
$created = $false
try {
    [void](New-Item -ItemType Directory -Path (Split-Path -Parent $Directory) -Force -ErrorAction Stop)
    [void](New-Item -ItemType Directory -Path $Directory -ErrorAction Stop)
    $created = $true
    Invoke-WebRequest -Uri $uri -Headers @{ 'JOB-TOKEN' = $env:CI_JOB_TOKEN } -OutFile $archive -MaximumRedirection 0 -TimeoutSec 90 -UseBasicParsing -ErrorAction Stop | Out-Null
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $Sha256) {
        throw 'Pinned Mise archive checksum differs from the mirrored artifact.'
    }
    Expand-Archive -LiteralPath $archive -DestinationPath $Directory -ErrorAction Stop
    $executable = Join-Path $Directory 'mise\bin\mise.exe'
    if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) {
        throw 'Pinned Mise executable is missing from the mirrored archive.'
    }
    $reported = & $executable --version
    if ($LASTEXITCODE -ne 0 -or $reported -notmatch ('^' + [regex]::Escape($Version) + '(\s|$)')) {
        throw 'Mirrored Mise executable reports an unexpected version.'
    }
    $env:PATH = (Split-Path -Parent $executable) + [IO.Path]::PathSeparator + $env:PATH
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    & icacls.exe $Directory /inheritance:r /grant:r "${identity}:(OI)(CI)F" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Cannot restrict Mise job directory ACL.' }
    $netrc = Join-Path $Directory '_netrc'
    [IO.File]::WriteAllText($netrc, "machine $env:CI_SERVER_HOST login gitlab-ci-token password $env:CI_JOB_TOKEN`n", [Text.UTF8Encoding]::new($false))
    & icacls.exe $netrc /inheritance:r /grant:r "${identity}:R" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Cannot restrict Mise mirror credential ACL.' }
    $mirrorBase = "$env:CI_API_V4_URL/projects/$env:CI_PROJECT_ID/$MirrorResource"
    $env:MISE_NETRC_FILE = $netrc
    $env:MISE_NETRC = 'true'
    $replacements = [ordered]@{}
    $replacements[$ReleaseMetadataPattern] = "${mirrorBase}${ReleaseMetadataResource}"
    $replacements['https://github.com/'] = $mirrorBase
    $replacements['https://api.github.com/'] = $mirrorBase
    $env:MISE_URL_REPLACEMENTS = $replacements | ConvertTo-Json -Compress
} catch {
    if ($created) {
        Remove-Item -LiteralPath $Directory -Recurse -Force -ErrorAction SilentlyContinue
    }
    throw
} finally {
    Remove-Item -LiteralPath $archive -Force -ErrorAction SilentlyContinue
}
