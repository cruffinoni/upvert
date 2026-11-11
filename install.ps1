# Install upvert from GitHub Releases on Windows.
#
# Usage:
#   irm https://raw.githubusercontent.com/dayrion/upvert/master/install.ps1 | iex
#   .\install.ps1 -Version vX.Y.Z -BinDir C:\tools\bin
#   .\install.ps1 -Local -BinDir C:\tools\bin
[CmdletBinding()]
param(
    [string]$Version,
    [string]$BinDir,
    [switch]$NoPathUpdate,
    [switch]$Local
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repo = 'dayrion/upvert'
$releasesUrl = "https://github.com/$repo/releases"
$downloadUrl = "$releasesUrl/download"

function Fail([string]$Message) {
    throw "upvert installer: $Message"
}

function Get-SourceRoot {
    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($PSScriptRoot)) {
        $candidates += $PSScriptRoot
    }
    $candidates += (Get-Location).Path
    foreach ($candidate in $candidates) {
        if ((Test-Path (Join-Path $candidate 'go.mod')) -and (Test-Path (Join-Path $candidate 'cmd\upvert'))) {
            return $candidate
        }
    }
    return $null
}

function Get-DefaultBinDir {
    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        Fail 'LOCALAPPDATA is not set; pass -BinDir'
    }
    return (Join-Path $env:LOCALAPPDATA 'Programs\upvert')
}

function Install-LocalBuild {
    $root = Get-SourceRoot
    if ($null -eq $root) {
        Fail 'could not find an upvert checkout; run this script from the repository root'
    }
    if ($null -eq (Get-Command go -CommandType Application -ErrorAction SilentlyContinue)) {
        Fail 'go is required for -Local'
    }

    if (-not [string]::IsNullOrWhiteSpace($BinDir)) {
        $targetDir = $BinDir
        $target = Join-Path $targetDir 'upvert.exe'
    }
    else {
        $command = Get-Command upvert -CommandType Application -ErrorAction SilentlyContinue
        if ($null -eq $command) {
            $command = Get-Command upvert.exe -CommandType Application -ErrorAction SilentlyContinue
        }
        if ($null -ne $command -and -not [string]::IsNullOrWhiteSpace($command.Source)) {
            $target = $command.Source
            $targetDir = Split-Path -Parent $target
        }
        else {
            $targetDir = Get-DefaultBinDir
            $target = Join-Path $targetDir 'upvert.exe'
        }
    }

    if (Test-Path -LiteralPath $target -PathType Container) {
        Fail "install target is a directory: $target"
    }
    New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
    $tmpBinary = Join-Path $targetDir ('.upvert-local-' + [Guid]::NewGuid().ToString('N') + '.exe')
    $oldCgo = $env:CGO_ENABLED
    try {
        Push-Location $root
        try {
            $env:CGO_ENABLED = '0'
            go build -trimpath -o $tmpBinary ./cmd/upvert
            if ($LASTEXITCODE -ne 0) {
                Fail 'go build failed'
            }
        }
        finally {
            Pop-Location
            if ($null -eq $oldCgo) {
                Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
            }
            else {
                $env:CGO_ENABLED = $oldCgo
            }
        }

        $backup = $target + '.old'
        Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue
        $hadTarget = Test-Path -LiteralPath $target -PathType Leaf
        if ($hadTarget) {
            Move-Item -LiteralPath $target -Destination $backup -Force
        }
        try {
            Move-Item -LiteralPath $tmpBinary -Destination $target -Force
        }
        catch {
            if ($hadTarget) {
                Move-Item -LiteralPath $backup -Destination $target -Force -ErrorAction SilentlyContinue
            }
            throw
        }
        Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue
        $tmpBinary = $null
        Write-Host "Installed upvert from source to $target"

        $onPath = $false
        foreach ($entry in @($env:Path -split ';')) {
            if ($entry.TrimEnd('\') -ieq $targetDir.TrimEnd('\')) {
                $onPath = $true
                break
            }
        }
        if (-not $onPath) {
            Write-Host ''
            Write-Host "Add $targetDir to your PATH to run upvert from other shells."
        }
    }
    finally {
        if ($null -ne $tmpBinary -and (Test-Path -LiteralPath $tmpBinary)) {
            Remove-Item -LiteralPath $tmpBinary -Force -ErrorAction SilentlyContinue
        }
    }
}


function Get-LatestTag {
    $headers = @{ 'User-Agent' = 'upvert-installer' }

    # Prefer the release-page redirect: it has no unauthenticated API rate limit.
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri "$releasesUrl/latest" -Headers $headers -MaximumRedirection 10
        $baseResponse = $response.BaseResponse
        $finalUri = $null
        if ($null -ne $baseResponse.PSObject.Properties['ResponseUri'] -and $null -ne $baseResponse.ResponseUri) {
            $finalUri = $baseResponse.ResponseUri.AbsoluteUri
        }
        elseif ($null -ne $baseResponse.PSObject.Properties['RequestMessage'] -and $null -ne $baseResponse.RequestMessage) {
            $finalUri = $baseResponse.RequestMessage.RequestUri.AbsoluteUri
        }
        if (-not [string]::IsNullOrWhiteSpace($finalUri)) {
            $tag = $finalUri.TrimEnd('/').Split('/')[-1]
            if ($tag -match '^v') {
                return $tag
            }
        }
    }
    catch {
        Write-Verbose "Could not resolve the latest release from the release page: $($_.Exception.Message)"
    }

    # Fall back to the API when a proxy or redirect policy hides the final URL.
    if (-not [string]::IsNullOrWhiteSpace($env:GITHUB_TOKEN)) {
        $headers['Authorization'] = "Bearer $($env:GITHUB_TOKEN)"
    }
    try {
        $release = Invoke-RestMethod -Uri "$releasesUrl/latest" -Headers $headers
        if (-not [string]::IsNullOrWhiteSpace($release.tag_name)) {
            return $release.tag_name
        }
    }
    catch {
        Fail "could not resolve the latest release: $($_.Exception.Message)"
    }

    Fail 'could not resolve the latest release'
}

if ($env:OS -ne 'Windows_NT') {
    Fail 'this installer supports Windows only; use install.sh on Linux or macOS'
}

if ($PSVersionTable.PSVersion.Major -lt 6) {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
}

if ($Local) {
    if ($PSBoundParameters.ContainsKey('Version')) {
        Fail '-Local cannot be combined with -Version'
    }
    Install-LocalBuild
    exit 0
}

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') {
    'arm64'
}
elseif ($env:PROCESSOR_ARCHITECTURE -eq 'AMD64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'AMD64') {
    'amd64'
}
else {
    Fail "unsupported CPU architecture: $env:PROCESSOR_ARCHITECTURE (supported: amd64, arm64)"
}

if ([string]::IsNullOrWhiteSpace($BinDir)) {
    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        Fail 'LOCALAPPDATA is not set; pass -BinDir'
    }
    $BinDir = Join-Path $env:LOCALAPPDATA 'Programs\upvert'
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    Write-Host 'Resolving the latest upvert release...'
    $Version = Get-LatestTag
}

if (-not $Version.StartsWith('v')) {
    $Version = "v$Version"
}

if ($Version -notmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$') {
    Fail "invalid release version: $Version"
}

$assetVersion = $Version.Substring(1)
$archive = "upvert_${assetVersion}_windows_${arch}.zip"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("upvert-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
    $archivePath = Join-Path $tmp $archive
    $checksumsPath = Join-Path $tmp 'checksums.txt'

    Write-Host "Downloading upvert $assetVersion for windows/$arch..."
    Invoke-WebRequest -UseBasicParsing -Uri "$downloadUrl/$Version/$archive" -OutFile $archivePath
    Invoke-WebRequest -UseBasicParsing -Uri "$downloadUrl/$Version/checksums.txt" -OutFile $checksumsPath

    $expected = $null
    foreach ($line in Get-Content -LiteralPath $checksumsPath) {
        $fields = $line -split '\s+'
        if ($fields.Count -ge 2 -and $fields[1] -eq $archive) {
            $expected = $fields[0].ToLowerInvariant()
            break
        }
    }
    if ([string]::IsNullOrWhiteSpace($expected)) {
        Fail "checksums.txt does not list $archive"
    }

    $actual = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        Fail "checksum mismatch for $archive"
    }

    $extractDir = Join-Path $tmp 'extract'
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDir -Force

    $binary = Join-Path $extractDir 'upvert.exe'
    if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) {
        $found = Get-ChildItem -LiteralPath $extractDir -Filter 'upvert.exe' -File -Recurse | Select-Object -First 1
        if ($null -eq $found) {
            Fail 'archive does not contain upvert.exe'
        }
        $binary = $found.FullName
    }

    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    $target = Join-Path $BinDir 'upvert.exe'
    Copy-Item -LiteralPath $binary -Destination $target -Force
    Write-Host "Installed upvert $Version to $target"

    $addToPath = -not $NoPathUpdate
    $pathUpdated = $false
    if ($addToPath) {
        $normalizedBin = $BinDir.TrimEnd('\')
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $userEntries = @()
        if (-not [string]::IsNullOrWhiteSpace($userPath)) {
            $userEntries = @($userPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
        }

        $alreadyInUserPath = $false
        foreach ($entry in $userEntries) {
            if ($entry.TrimEnd('\') -ieq $normalizedBin) {
                $alreadyInUserPath = $true
                break
            }
        }
        if (-not $alreadyInUserPath) {
            $newUserPath = (@($userEntries) + $BinDir) -join ';'
            [Environment]::SetEnvironmentVariable('Path', $newUserPath, 'User')
            $pathUpdated = $true
            Write-Host "Added $BinDir to the user PATH."
        }

        $alreadyInProcessPath = $false
        foreach ($entry in @($env:Path -split ';')) {
            if ($entry.TrimEnd('\') -ieq $normalizedBin) {
                $alreadyInProcessPath = $true
                break
            }
        }
        if (-not $alreadyInProcessPath) {
            $env:Path = "$BinDir;$env:Path"
        }
    }

    if ($pathUpdated) {
        Write-Host 'Restart your terminal to use upvert from other shells.'
    }
    elseif (-not $addToPath) {
        Write-Host "Add $BinDir to your PATH to run upvert from other shells."
    }

    Write-Host ''
    Write-Host 'Verify the installation:'
    Write-Host '  upvert version'
}
finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
