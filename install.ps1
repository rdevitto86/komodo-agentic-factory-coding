# Downloads this machine's komodo release, verifies its SHA-256, and hands every other step to komodo install.
# Run: powershell -ExecutionPolicy Bypass -File install.ps1. Environment: KOMODO_VERSION, KOMODO_RELEASE_URL.
$ErrorActionPreference = 'Stop'
$base = "$(if ($env:KOMODO_RELEASE_URL) { $env:KOMODO_RELEASE_URL } else { 'https://github.com/rdevitto86/komodo-agentic-factory-coding/releases/download' })/$(if ($env:KOMODO_VERSION) { $env:KOMODO_VERSION } else { 'v1.0.0-beta.2' })"
$name = "komodo-windows-$(if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }).exe"
$dir = Join-Path $HOME '.komodo\bin'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Invoke-WebRequest -UseBasicParsing -Uri "$base/$name" -OutFile "$dir\$name.new"
$want = ((Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS").Content -split "`n" | Where-Object { ($_ -split '\s+')[1] -in @($name, "*$name") } | ForEach-Object { ($_ -split '\s+')[0] })
$got = (Get-FileHash -Algorithm SHA256 -Path "$dir\$name.new").Hash
if (-not $want -or $got -ne $want) { Remove-Item "$dir\$name.new"; throw "install: checksum mismatch for $name; nothing was installed" }
Move-Item -Force -Path "$dir\$name.new" -Destination "$dir\$name"
[Environment]::SetEnvironmentVariable('Path', ((@([Environment]::GetEnvironmentVariable('Path', 'User') -split ';') + $dir | Where-Object { $_ } | Select-Object -Unique) -join ';'), 'User')
& "$dir\$name" install
exit $LASTEXITCODE
