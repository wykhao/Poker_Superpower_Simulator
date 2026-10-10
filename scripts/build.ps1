param([string]$Go='go',[ValidatePattern('^\d+\.\d+\.\d+([-.][A-Za-z0-9.]+)?$')][string]$Version='0.1.0')
$ErrorActionPreference='Stop'
$projectRoot=Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    $targetOS=(& $Go env GOOS).Trim()
    $name=if($targetOS -eq 'windows'){'poker-simulator.exe'}else{'poker-simulator'}
    $linkFlags="-s -w -X poker-human-simulator/internal/app.Version=$Version"
    if($targetOS -eq 'windows'){$linkFlags+=' -H=windowsgui'}
    New-Item -ItemType Directory -Path (Join-Path $projectRoot 'bin') -Force | Out-Null
    & $Go build -trimpath -buildvcs=false -ldflags $linkFlags -o (Join-Path $projectRoot "bin\$name") ./cmd/poker-simulator
    if($LASTEXITCODE -ne 0){throw 'Build failed'}
    Write-Output (Join-Path $projectRoot "bin\$name")
} finally {Pop-Location}
