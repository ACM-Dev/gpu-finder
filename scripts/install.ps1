$ErrorActionPreference = "Stop"

$Repo = "ACM-Dev/gpu-finder"
$BinName = "gpu-finder.exe"

# Fetch latest version from GitHub API
$LatestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
$Version = $LatestRelease.tag_name
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version"

$InstallDir = Join-Path $env:LOCALAPPDATA "gpu-finder"
$BinPath = Join-Path $InstallDir $BinName

# Uninstall mode
if ($args[0] -eq "--uninstall") {
    if (-not (Test-Path $BinPath)) {
        Write-Host "❌ gpu-finder not found at $BinPath"
        exit 1
    }

    Write-Host "🗑️  Uninstalling gpu-finder..."
    Remove-Item $BinPath -Force
    Write-Host "✅ Removed $BinPath"

    # Remove from user PATH
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -like "*$InstallDir*") {
        Write-Host "🛤️  Removing $InstallDir from user PATH..."
        $newPath = ($userPath -split ';' | Where-Object { $_ -ne $InstallDir }) -join ';'
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = $newPath
        Write-Host "✅ Removed from PATH"
    }

    Write-Host ""
    Write-Host "🔄 Uninstall complete. Close and reopen your terminal."
    exit 0
}

$OS = "windows"
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}

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
Write-Host "📦 Downloading $FileName ($Version)..."
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
Write-Host "✅ Installed $BinName $Version to $BinPath"
Write-Host ""
Write-Host "🔄 To use gpu-finder:"
Write-Host "   • Restart PowerShell, or close and reopen your terminal"
Write-Host "   • Then run:                    gpu-finder"
Write-Host "   • To uninstall:                irm https://github.com/ACM-Dev/gpu-finder/raw/main/scripts/install.ps1 | iex -ArgumentList '--uninstall'"
Write-Host ""
& $BinPath $args
