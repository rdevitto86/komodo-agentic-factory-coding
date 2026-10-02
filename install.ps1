# Installs komodo on native Windows; running it again updates it in place.
# Run: powershell -ExecutionPolicy Bypass -File install.ps1, from a komodo checkout or once downloaded.
# It names any missing prerequisite, builds with Go from the checkout or downloads the pinned release
# and verifies its SHA-256, writes a komodo.cmd wrapper onto the user PATH, since symlinks need admin
# rights, runs komodo install --global, then komodo init and komodo doctor inside a repo.
# Environment: KOMODO_VERSION, KOMODO_RELEASE_URL, KOMODO_BIN_DIR (default %LOCALAPPDATA%\komodo\bin).
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Version = if ($env:KOMODO_VERSION) { $env:KOMODO_VERSION } else { 'v1.0.0-beta.2' }
$ReleaseUrl = if ($env:KOMODO_RELEASE_URL) { $env:KOMODO_RELEASE_URL } else {
    'https://github.com/rdevitto86/komodo-agentic-factory-coding/releases/download'
}
$BinDir = if ($env:KOMODO_BIN_DIR) { $env:KOMODO_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'komodo\bin' }
$WorkDir = Join-Path ([IO.Path]::GetTempPath()) ('komodo-install-' + [Guid]::NewGuid().ToString('N'))

function Say([string]$Message) {
    Write-Host "install: $Message"
}

function Fail([string]$Message) {
    [Console]::Error.WriteLine("install: $Message")
    exit 1
}

function Invoke-Checked([string]$Dir, [string]$Exe, [string[]]$Arguments) {
    Push-Location $Dir
    try {
        & $Exe @Arguments | Out-Host
        return $LASTEXITCODE
    } finally {
        Pop-Location
    }
}

function Get-Arch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { Fail "$($env:PROCESSOR_ARCHITECTURE) has no komodo build" }
    }
}

function Test-Prerequisites {
    $missing = $false
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        $missing = $true
        [Console]::Error.WriteLine('install: git is missing; run winget install Git.Git, or see https://git-scm.com/download/win')
    }
    if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
        $missing = $true
        [Console]::Error.WriteLine('install: Claude Code is missing; run npm install -g @anthropic-ai/claude-code, ' +
            'or see https://docs.claude.com/en/docs/claude-code/setup')
    }
    if ($missing) {
        exit 1
    }
}

function Build-Binary([string]$Name) {
    Say "building $Name from $PSScriptRoot"
    New-Item -ItemType Directory -Force -Path (Join-Path $PSScriptRoot 'bin') | Out-Null
    if ((Invoke-Checked $PSScriptRoot 'go' @('build', '-o', "bin\$Name", './cmd/komodo')) -ne 0) {
        Fail "go build failed"
    }
    return Join-Path $PSScriptRoot "bin\$Name"
}

function Get-Release([string]$Name) {
    $base = "$ReleaseUrl/$Version"
    $dir = Join-Path $HOME '.komodo\bin'
    $asset = Join-Path $WorkDir $Name
    $sums = Join-Path $WorkDir 'SHA256SUMS'
    Say "downloading $Name $Version"
    try {
        Invoke-WebRequest -UseBasicParsing -Uri "$base/$Name" -OutFile $asset
        Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile $sums
    } catch {
        Fail "could not download $Version from ${base}: $($_.Exception.Message)"
    }
    $want = $null
    foreach ($line in Get-Content $sums) {
        $fields = $line.Trim() -split '\s+'
        if ($fields.Count -ge 2 -and $fields[1].TrimStart('*') -eq $Name) {
            $want = $fields[0]
        }
    }
    if (-not $want) {
        Fail "SHA256SUMS for $Version lists no $Name"
    }
    $got = (Get-FileHash -Algorithm SHA256 -Path $asset).Hash
    if ($got -ne $want) {
        Fail "checksum mismatch for ${Name}: got $got, want $want; nothing was installed"
    }
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Move-Item -Force -Path $asset -Destination (Join-Path $dir $Name)
    return Join-Path $dir $Name
}

function Set-Wrapper([string]$Binary) {
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $wrapper = Join-Path $BinDir 'komodo.cmd'
    Set-Content -Path $wrapper -Encoding ASCII -Value "@echo off`r`n`"$Binary`" %*`r`n"
    Say "wrote $wrapper -> $Binary"
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @()
    if ($userPath) {
        $entries = $userPath -split ';'
    }
    if ($entries -notcontains $BinDir) {
        [Environment]::SetEnvironmentVariable('Path', (($entries + $BinDir) -join ';').Trim(';'), 'User')
        Say "added $BinDir to the user PATH; open a new terminal to pick it up"
    }
    if (($env:Path -split ';') -notcontains $BinDir) {
        $env:Path = "$BinDir;$env:Path"
    }
    return $wrapper
}

function Get-RepoRoot([string]$Dir) {
    # Windows PowerShell turns a native command's stderr into a terminating error under Stop.
    $ErrorActionPreference = 'Continue'
    Push-Location $Dir
    try {
        $top = & git rev-parse --show-toplevel 2>$null
        if ($LASTEXITCODE -eq 0 -and $top) {
            return $top.Trim()
        }
        return $null
    } finally {
        Pop-Location
    }
}

New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null
try {
    Test-Prerequisites
    $name = "komodo-windows-$(Get-Arch).exe"
    $repo = Get-RepoRoot (Get-Location).Path
    if ((Get-Command go -ErrorAction SilentlyContinue) -and (Test-Path (Join-Path $PSScriptRoot 'cmd\komodo\main.go'))) {
        $binary = Build-Binary 'komodo.exe'
    } else {
        $binary = Get-Release $name
    }
    $komodo = Set-Wrapper $binary

    if (-not $repo) {
        # komodo runs inside a git repository, so the global install runs in the checkout, else an empty one.
        $toolkit = Get-RepoRoot $PSScriptRoot
        if (-not $toolkit) {
            $toolkit = Join-Path $WorkDir 'empty'
            & git init -q $toolkit | Out-Host
        }
        if ((Invoke-Checked $toolkit $komodo @('install', '--global')) -ne 0) {
            Fail 'komodo install --global failed'
        }
        Say 'done; run komodo init inside a repo to add the line to it'
        exit 0
    }
    if ((Invoke-Checked $repo $komodo @('install', '--global')) -ne 0) {
        Fail 'komodo install --global failed'
    }
    if ((Invoke-Checked $repo $komodo @('init')) -ne 0) {
        Fail 'komodo init failed'
    }
    if ((Invoke-Checked $repo $komodo @('doctor')) -ne 0) {
        Say 'komodo doctor found the problems above; fix them and run komodo doctor again'
    }
    Say 'done'
} finally {
    Remove-Item -Recurse -Force -Path $WorkDir -ErrorAction SilentlyContinue
}
