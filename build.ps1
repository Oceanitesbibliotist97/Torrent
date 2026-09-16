<#
.SYNOPSIS
  Builds the portable Torrent.exe.

.DESCRIPTION
  Produces a single self-contained executable in build\bin together with a
  SHA-256 checksum file. The build is pure Go (CGO_ENABLED=0) and uses
  -trimpath, so the binary contains no paths from the build machine and can be
  reproduced by anyone with the same Go version.

.EXAMPLE
  .\build.ps1 -Version 1.0.0
#>
param(
  [string]$Version = "1.0.0",
  [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

function Invoke-Step([string]$Name, [scriptblock]$Body) {
  Write-Host "==> $Name" -ForegroundColor Cyan
  & $Body
  if ($LASTEXITCODE -ne 0) { throw "$Name failed" }
}

if (-not $SkipTests) {
  Invoke-Step "go vet" { go vet ./... }
  Invoke-Step "go test" { go test ./... }
}

Invoke-Step "Windows resources (icon, manifest, version info)" {
  go run ./tools/winres -version $Version
}

New-Item -ItemType Directory -Force build\bin | Out-Null
$ldflags = "-s -w -H windowsgui -buildid= -X github.com/ClearNetSky/Torrent/internal/appinfo.Version=$Version"
Invoke-Step "go build" {
  go build -tags desktop,production -trimpath -buildvcs=false -ldflags $ldflags -o build\bin\Torrent.exe .
}

$exe = Get-Item build\bin\Torrent.exe
$hash = (Get-FileHash $exe.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
"$hash  Torrent.exe" | Set-Content -Encoding ascii build\bin\Torrent.exe.sha256

Write-Host ""
Write-Host ("Built {0} ({1:N1} MB)" -f $exe.FullName, ($exe.Length / 1MB)) -ForegroundColor Green
Write-Host "SHA-256 $hash"
