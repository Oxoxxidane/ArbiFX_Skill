param([string]$Go = 'go', [string]$OutputDir = (Join-Path $env:USERPROFILE 'Downloads\arbifx-http-release'))
$ErrorActionPreference = 'Stop'
$skillRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
& $Go run (Join-Path $PSScriptRoot 'release.go') -root $skillRoot -out $OutputDir
if ($LASTEXITCODE -ne 0) { throw 'Build or validation failed.' }
