#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [switch]$SkipTypeScriptCheck,
    [switch]$SkipTailwindBuild,
    [switch]$SkipStyleAudit,
    [switch]$SkipGoTest,
    [switch]$Deploy,
    [string]$DeployUser = "",
    [string]$DeployHost = "",
    [int]$DeployPort = 22,
    [string]$DeployPath = "/opt/nxsandbox",
    [string]$DeployService = "nxsandbox"
)

$ErrorActionPreference = "Stop"

function Write-Step([string]$msg) {
    Write-Host "`n=== $msg ===" -ForegroundColor Cyan
}

function Invoke-Checked([string]$description, [scriptblock]$command) {
    Write-Host "-> $description" -ForegroundColor DarkCyan
    $ErrorActionPreference = 'Continue'
    & $command
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
    if ($exitCode -ne 0) {
        throw "$description failed (exit $exitCode)"
    }
}

function Get-RepoRoot {
    $root = Split-Path -Parent $MyInvocation.PSCommandPath
    return (Resolve-Path $root).Path
}

function Read-PackageJson([string]$path) {
    if (-not (Test-Path $path)) { return $null }
    $raw = Get-Content -Path $path -Raw
    if ([string]::IsNullOrWhiteSpace($raw)) { return $null }
    return ($raw | ConvertFrom-Json)
}

function Ensure-TailwindDependencies([string]$repoRootPath) {
    $rootPkgPath = Join-Path $repoRootPath "package.json"
    if (-not (Test-Path $rootPkgPath)) {
        Write-Host "No root package.json found. Creating minimal build package..." -ForegroundColor Yellow
        $rootPkg = @"
{
  "name": "nxsandbox-build-tools",
  "private": true,
  "version": "0.0.0",
  "description": "Local build dependencies for nxsandbox",
  "license": "UNLICENSED"
}
"@
        Set-Content -Path $rootPkgPath -Value $rootPkg
    }

    $rootPkgJson = Read-PackageJson $rootPkgPath
    $hasTailwind = $false
    $hasCli = $false
    if ($rootPkgJson -and $rootPkgJson.devDependencies) {
        $hasTailwind = [bool]$rootPkgJson.devDependencies.tailwindcss
        $hasCli = [bool]$rootPkgJson.devDependencies."@tailwindcss/cli"
    }

    if ($hasTailwind -and $hasCli) {
        return
    }

    Invoke-Checked "Install Tailwind local build deps" { npm install --save-dev --no-audit --no-fund tailwindcss @tailwindcss/cli }
}

$repoRoot = Get-RepoRoot
Set-Location $repoRoot

Write-Step "Preflight"
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is required but was not found in PATH"
}
if ((-not $SkipTypeScriptCheck) -or (-not $SkipTailwindBuild)) {
    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
        throw "npm is required for TypeScript/Tailwind checks but was not found in PATH"
    }
}

$versionFile = Join-Path $repoRoot "version.txt"
$buildNumFile = Join-Path $repoRoot "build_number.txt"
if (-not (Test-Path $versionFile)) {
    "0.1.0" | Set-Content -Path $versionFile -NoNewline
}
if (-not (Test-Path $buildNumFile)) {
    "0" | Set-Content -Path $buildNumFile -NoNewline
}
$version = (Get-Content $versionFile -Raw).Trim()
$buildNum = [int](Get-Content $buildNumFile -Raw).Trim() + 1
$buildTime = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss")
$buildNum | Set-Content -Path $buildNumFile -NoNewline

Write-Host "Version: $version (Build #$buildNum)" -ForegroundColor Magenta

if (-not $SkipTypeScriptCheck) {
    Write-Step "TypeScript syntax checks"

    $packageFiles = Get-ChildItem -Path $repoRoot -Recurse -Filter package.json -File |
        Where-Object { $_.FullName -notmatch "[\\/]node_modules[\\/]" }

    if ($packageFiles.Count -eq 0) {
        Write-Host "No package.json files found. TypeScript check skipped." -ForegroundColor Yellow
    } else {
        foreach ($pkg in $packageFiles) {
            $pkgDir = Split-Path -Parent $pkg.FullName
            $tsconfigPath = Join-Path $pkgDir "tsconfig.json"
            if (-not (Test-Path $tsconfigPath)) {
                Write-Host "No tsconfig.json in $pkgDir. Skipping." -ForegroundColor Yellow
                continue
            }

            Push-Location $pkgDir
            try {
                $pkgJson = Read-PackageJson $pkg.FullName
                $hasTypecheckScript = $false
                if ($pkgJson -and $pkgJson.scripts -and $pkgJson.scripts.typecheck) {
                    $hasTypecheckScript = $true
                }

                if ($hasTypecheckScript) {
                    Invoke-Checked "npm run typecheck in $pkgDir" { npm run typecheck }
                } else {
                    Invoke-Checked "npx tsc --noEmit in $pkgDir" { npx tsc --noEmit }
                }
            } finally {
                Pop-Location
            }
        }
    }
}

if (-not $SkipTailwindBuild) {
    Write-Step "Tailwind local CSS build"

    Ensure-TailwindDependencies $repoRoot

    $tailwindInput = Join-Path $repoRoot "styles\ssr-input.css"
    $tailwindOutput = Join-Path $repoRoot "internal\web\static\css\ssr.css"
    if (-not (Test-Path $tailwindInput)) {
        throw "Tailwind input is missing: $tailwindInput"
    }

    $outputDir = Split-Path -Parent $tailwindOutput
    if (-not (Test-Path $outputDir)) {
        New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
    }

    Invoke-Checked "Build local tailwind css" { npx @tailwindcss/cli -i $tailwindInput -o $tailwindOutput --minify }
    if (-not (Test-Path $tailwindOutput)) {
        throw "Tailwind output was not created: $tailwindOutput"
    }
}

if (-not $SkipStyleAudit) {
    Write-Step "Style audit: no CDN tailwind and mostly inline utility classes"

    $templateDir = Join-Path $repoRoot "internal\web\templates"
    if (-not (Test-Path $templateDir)) {
        throw "Template directory not found: $templateDir"
    }

    $templates = Get-ChildItem -Path $templateDir -Filter *.html -File
    if ($templates.Count -eq 0) {
        throw "No HTML templates found under $templateDir"
    }

    $cdnPattern = '(?i)(cdn\.tailwindcss\.com|unpkg\.com/.+tailwind|jsdelivr\.net/.+tailwind)'
    $styleTagPattern = '(?i)<style[\s>]'

    $totalClassAttrs = 0
    foreach ($tpl in $templates) {
        $content = Get-Content -Path $tpl.FullName -Raw

        if ($content -match $cdnPattern) {
            throw "Tailwind CDN reference found in $($tpl.FullName)"
        }
        if ($content -match $styleTagPattern) {
            throw "Inline <style> block found in $($tpl.FullName). Prefer inline utility classes with local tailwind build."
        }
        if ($content -notmatch '/static/css/ssr\.css') {
            throw "Missing local css link (/static/css/ssr.css) in $($tpl.FullName)"
        }

        $classMatches = [regex]::Matches($content, 'class\s*=\s*"[^"]+"')
        $totalClassAttrs += $classMatches.Count
    }

    if ($totalClassAttrs -eq 0) {
        throw "No class attributes found in templates. Expected inline tailwind utility classes."
    }

    Write-Host "Style audit passed. class attributes found: $totalClassAttrs" -ForegroundColor Green
}

Write-Step "Go module and tests"
Invoke-Checked "go mod tidy" { go mod tidy }
if (-not $SkipGoTest) {
    Invoke-Checked "go test ./..." { go test ./... }
}

$ldFlags = "-s -w -X main.Version=$version -X main.BuildTime=$buildTime -X main.BuildNum=$buildNum"

Write-Step "Build binaries with CGO disabled"
$env:CGO_ENABLED = "0"

Invoke-Checked "Build Windows binary (amd64)" {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -o ./nxsandbox.exe -ldflags $ldFlags ./cmd/nxsandbox
}

Invoke-Checked "Build Linux binary (amd64)" {
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    go build -o ./nxsandbox-linux-new -ldflags $ldFlags ./cmd/nxsandbox
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue

if ($Deploy) {
    Write-Step "Deploy"
    if ([string]::IsNullOrWhiteSpace($DeployHost) -or [string]::IsNullOrWhiteSpace($DeployUser)) {
        throw "Deploy requires -DeployHost and -DeployUser"
    }

    $remoteTmp = "/tmp/nxsandbox-linux-new"
    $remoteDst = "$DeployPath/nxsandbox"

    Invoke-Checked "Copy Linux binary to remote" {
        scp -P $DeployPort ./nxsandbox-linux-new "$DeployUser@$DeployHost`:$remoteTmp"
    }

    Invoke-Checked "Install binary and restart service" {
        ssh -p $DeployPort "$DeployUser@$DeployHost" "set -e; sudo install -m 755 $remoteTmp $remoteDst; sudo systemctl restart $DeployService; sudo systemctl status $DeployService --no-pager --lines=20"
    }
}

Write-Host "`nBuild complete." -ForegroundColor Cyan
Write-Host "Artifacts:" -ForegroundColor Cyan
Write-Host "  ./nxsandbox.exe"
Write-Host "  ./nxsandbox-linux-new"
