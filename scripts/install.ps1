param([string]$BinDir = (Join-Path $env:LOCALAPPDATA 'ArbiFX-CLI\bin'))
$ErrorActionPreference = 'Stop'
$cpu = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') { 'arm64' } else { 'amd64' }
$source = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..\bin\windows-$cpu\arbifx.exe")).Path
$existing = Get-Command arbifx -ErrorAction SilentlyContinue
$destination = [IO.Path]::GetFullPath((Join-Path $BinDir 'arbifx.exe'))
if ($existing -and $existing.Source -ne $destination -and $existing.Source -ne $source) {
    throw "The arbifx command name is already in use: $($existing.Source)"
}
New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
if ($source -ne $destination) { Copy-Item -LiteralPath $source -Destination $destination -Force }
# Add only the current user's PATH entry; no system PATH changes or administrator access.
$absoluteBin = [IO.Path]::GetDirectoryName($destination)
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($absoluteBin -notin ($userPath -split ';')) {
    $newPath = if ([string]::IsNullOrEmpty($userPath)) { $absoluteBin } else { "$userPath;$absoluteBin" }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
}
if ($absoluteBin -notin ($env:Path -split ';')) { $env:Path += ";$absoluteBin" }
& $destination --version
if ($LASTEXITCODE -ne 0) { throw 'The installed executable failed to start.' }
Write-Output "Installed: $destination (restart other open terminals to pick up the user PATH)"
