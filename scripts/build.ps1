$ErrorActionPreference = 'Stop'

$Root = Split-Path -Parent $PSScriptRoot
$Desktop = Join-Path $Root 'cmd\frp-client-desktop'
$IconSource = Join-Path $Desktop 'assets\appicon.png'
$BuildDir = Join-Path $Desktop 'build'
$AppIcon = Join-Path $BuildDir 'appicon.png'
$WindowsIcon = Join-Path $BuildDir 'windows\icon.ico'

New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null
Copy-Item -LiteralPath $IconSource -Destination $AppIcon -Force
if (Test-Path $WindowsIcon) {
    Remove-Item -LiteralPath $WindowsIcon -Force
}

Push-Location $Root
try {
    node --check cmd/frp-client-desktop/frontend/app.js
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    node scripts/check-frontend.mjs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    go test ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Push-Location $Desktop
    try {
        wails build -clean
    }
    finally {
        Pop-Location
    }
}
finally {
    Pop-Location
}

Write-Host ''
Write-Host 'Build complete:'
Write-Host (Join-Path $Desktop 'build\bin\frp-client-manager.exe')
