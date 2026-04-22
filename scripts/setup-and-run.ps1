# GPU Capacity Finder — Setup & Run Script (PowerShell)
# Run: .\setup-and-run.ps1

$ErrorActionPreference = "Stop"

function Check-Command {
    param([string]$Name)
    $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  GPU Capacity Finder — Setup & Run" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# --- Check Python ---
$pythonCmd = $null
if (Check-Command "python") {
    $pythonCmd = "python"
} elseif (Check-Command "python3") {
    $pythonCmd = "python3"
}

if (-not $pythonCmd) {
    Write-Host "[!] Python not found." -ForegroundColor Red
    Write-Host ""
    Write-Host "Install Python from the Microsoft Store (recommended):" -ForegroundColor Yellow
    Write-Host "  ms-windows-store://pdp/?ProductId=9NRWMJP3717K" -ForegroundColor White
    Write-Host ""
    Write-Host "Or download from: https://www.python.org/downloads/windows/" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "After installing, restart this script." -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

$pyVersion = & $pythonCmd --version 2>&1
Write-Host "[OK] Found $pyVersion" -ForegroundColor Green

# --- Check uv ---
if (-not (Check-Command "uv")) {
    Write-Host ""
    Write-Host "[!] uv not found." -ForegroundColor Red
    Write-Host ""
    Write-Host "Install uv with one of these commands:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  Option 1 (PowerShell):" -ForegroundColor White
    Write-Host "    irm https://astral.sh/uv/install.ps1 | iex" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  Option 2 (pip):" -ForegroundColor White
    Write-Host "    $pythonCmd -m pip install uv" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  Option 3 (winget):" -ForegroundColor White
    Write-Host "    winget install --id=astral-sh.uv -e" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "More info: https://docs.astral.sh/uv/getting-started/installation/" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "After installing uv, restart this script." -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

$uvVersion = & uv --version 2>&1
Write-Host "[OK] Found uv $uvVersion" -ForegroundColor Green

# --- Run the tool ---
Write-Host ""
Write-Host "Starting GPU Capacity Finder..." -ForegroundColor Cyan
Write-Host ""

uv run --with 'boto3[crt]' --with textual --with rich $pythonCmd gpu_capacity_finder.py
