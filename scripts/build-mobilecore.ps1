param(
    [string]$Version = "0.4.0",
    [int]$ApiSchemaVersion = 1,
    [int]$AndroidApi = 24,
    [string]$Target = "android/arm64",
    [string]$OutputDir = "release",
    [switch]$SkipAAR
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$ReleaseDir = Join-Path $RepoRoot $OutputDir
$AarName = "yatori-mobile-v$Version.aar"
$AarPath = Join-Path $ReleaseDir $AarName
$SchemaSource = Join-Path $RepoRoot "mobilecore\api-schema.json"
$SchemaTarget = Join-Path $ReleaseDir "api-schema.json"
$VersionTarget = Join-Path $ReleaseDir "yatori-core-version.json"
$ChecksumTarget = Join-Path $ReleaseDir "yatori-mobilecore-checksums.json"

Set-Location $RepoRoot
New-Item -ItemType Directory -Force $ReleaseDir | Out-Null

Write-Host ""
Write-Host "====================================================" -ForegroundColor Cyan
Write-Host "  Build yatori mobile core AAR" -ForegroundColor Cyan
Write-Host "====================================================" -ForegroundColor Cyan

Write-Host "[1/5] gofmt and test mobilecore..." -ForegroundColor Yellow
gofmt -w mobilecore
go test ./mobilecore -count=1 -v
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "[2/5] Copy API schema..." -ForegroundColor Yellow
$Schema = Get-Content $SchemaSource -Raw | ConvertFrom-Json
if ([int]$Schema.schemaVersion -ne $ApiSchemaVersion) {
    throw "api-schema.json schemaVersion=$($Schema.schemaVersion), but -ApiSchemaVersion=$ApiSchemaVersion"
}
Copy-Item $SchemaSource $SchemaTarget -Force

$Commit = "unknown"
try {
    $Commit = (git rev-parse --short HEAD).Trim()
} catch {}

$BuiltAt = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
Write-Host "[3/5] Prepare core version metadata..." -ForegroundColor Yellow

if ($SkipAAR) {
    Write-Host "[4/5] Skip AAR build by -SkipAAR" -ForegroundColor Yellow
} else {
    Write-Host "[4/5] Build AAR with gomobile bind..." -ForegroundColor Yellow
    $env:PATH = "$($env:USERPROFILE)\go\bin;$($env:PATH)"
    $TargetArg = "-target=$Target"
    gomobile bind $TargetArg "-androidapi=$AndroidApi" -trimpath "-ldflags=-s -w" -o $AarPath ./mobilecore
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "[5/5] Write metadata and check artifacts..." -ForegroundColor Yellow
if (-not $SkipAAR -and -not (Test-Path $AarPath)) {
    throw "AAR missing: $AarPath"
}
if (-not (Test-Path $SchemaTarget)) {
    throw "api-schema.json missing: $SchemaTarget"
}
if (-not (Test-Path $VersionTarget)) {
    New-Item -ItemType File -Force $VersionTarget | Out-Null
}

$AarSha256 = ""
$AarBytes = 0
if (Test-Path $AarPath) {
    $AarSha256 = (Get-FileHash $AarPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $AarBytes = (Get-Item $AarPath).Length
}
$SchemaSha256 = (Get-FileHash $SchemaTarget -Algorithm SHA256).Hash.ToLowerInvariant()

$CoreVersion = [ordered]@{
    androidVersion = $Version
    desktopCoreVersion = $Version
    coreCommit = $Commit
    apiSchemaVersion = $ApiSchemaVersion
    aarFile = $AarName
    aarSha256 = $AarSha256
    aarBytes = $AarBytes
    target = $Target
    androidApi = $AndroidApi
    builtAt = $BuiltAt
}
$CoreVersion | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 $VersionTarget
$VersionSha256 = (Get-FileHash $VersionTarget -Algorithm SHA256).Hash.ToLowerInvariant()

$ChecksumFiles = [ordered]@{}
$ChecksumFiles[$AarName] = $AarSha256
$ChecksumFiles["api-schema.json"] = $SchemaSha256
$ChecksumFiles["yatori-core-version.json"] = $VersionSha256

$Checksums = [ordered]@{
    generatedAt = $BuiltAt
    desktopCoreVersion = $Version
    coreCommit = $Commit
    apiSchemaVersion = $ApiSchemaVersion
    files = $ChecksumFiles
}
$Checksums | ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 $ChecksumTarget

Write-Host ""
Write-Host "Done." -ForegroundColor Green
if (Test-Path $AarPath) { Write-Host "  AAR    : $AarPath" -ForegroundColor White }
Write-Host "  Schema : $SchemaTarget" -ForegroundColor White
Write-Host "  Version: $VersionTarget" -ForegroundColor White
Write-Host "  Hashes : $ChecksumTarget" -ForegroundColor White

