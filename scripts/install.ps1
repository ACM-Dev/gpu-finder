$ErrorActionPreference = "Stop"

$Repo = "ACM-Dev/gpu-finder"
$Version = "v1.0.0"
$BinName = "gpu-finder.exe"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version"

$OS = "windows"
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}

$InstallDir = Join-Path $env:LOCALAPPDATA "gpu-finder"
$BinPath = Join-Path $InstallDir $BinName

if (Get-Command $BinName -ErrorAction SilentlyContinue) {
    Write-Host "✅ $BinName already installed: $(Get-Command $BinName | Select-Object -ExpandProperty Source)"
    Write-Host ""
    $response = Read-Host "Re-download $Version? [y/N]"
    if ($response -ne "y" -and $response -ne "Y") {
        & gpu-finder $args
        exit 0
    }
}

if (Test-Path ".\$BinName") {
    Write-Host "✅ Found local binary: .\$BinName"
    Write-Host ""
    $response = Read-Host "Use local? [Y/n]"
    if ($response -ne "n" -and $response -ne "N") {
        & ".\$BinName" $args
        exit 0
    }
}

$FileName = "gpu-finder-$Version-$OS-$Arch.tar.gz"
$ArchiveBin = "gpu-finder-$OS-$Arch.exe"
Write-Host "📦 Downloading $FileName..."
Invoke-WebRequest -Uri "$DownloadUrl/$FileName" -OutFile "$env:TEMP\$FileName" -UseBasicParsing

Write-Host "📂 Extracting..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
tar xzf "$env:TEMP\$FileName" -C "$env:TEMP"

Write-Host "🔧 Renaming to $BinName..."
Move-Item "$env:TEMP\$ArchiveBin" $BinPath -Force
Remove-Item "$env:TEMP\$FileName" -Force

# Add to user PATH if not already present
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    Write-Host "🛤️  Adding $InstallDir to user PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
}

Write-Host ""
Write-Host "✅ Installed to $BinPath"
Write-Host ""
& $BinPath $args
