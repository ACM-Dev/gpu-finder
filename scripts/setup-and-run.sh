#!/usr/bin/env bash
# GPU Capacity Finder — Setup & Run Script (Linux/macOS)
# Run: chmod +x setup-and-run.sh && ./setup-and-run.sh

set -e

echo "========================================"
echo "  GPU Capacity Finder — Setup & Run"
echo "========================================"
echo ""

# --- Check Python ---
PYTHON_CMD=""
if command -v python3 &>/dev/null; then
    PYTHON_CMD="python3"
elif command -v python &>/dev/null; then
    PYTHON_CMD="python"
fi

if [ -z "$PYTHON_CMD" ]; then
    echo "[!] Python not found."
    echo ""
    echo "Install Python:"
    echo ""
    echo "  Ubuntu/Debian:  sudo apt install python3 python3-pip"
    echo "  Fedora:         sudo dnf install python3"
    echo "  Arch:           sudo pacman -S python"
    echo "  macOS:          brew install python3"
    echo ""
    echo "  Or download:    https://www.python.org/downloads/"
    echo ""
    echo "After installing, restart this script."
    exit 1
fi

PY_VERSION=$("$PYTHON_CMD" --version 2>&1)
echo "[OK] Found $PY_VERSION"

# --- Check uv ---
if ! command -v uv &>/dev/null; then
    echo ""
    echo "[!] uv not found."
    echo ""
    echo "Install uv with one of these commands:"
    echo ""
    echo "  Option 1 (official installer):"
    echo "    curl -LsSf https://astral.sh/uv/install.sh | sh"
    echo ""
    echo "  Option 2 (pip):"
    echo "    $PYTHON_CMD -m pip install uv"
    echo ""
    echo "  Option 3 (Homebrew, macOS):"
    echo "    brew install uv"
    echo ""
    echo "  Option 4 (pipx):"
    echo "    pipx install uv"
    echo ""
    echo "  More info:    https://docs.astral.sh/uv/getting-started/installation/"
    echo ""
    echo "After installing uv, restart this script."
    exit 1
fi

UV_VERSION=$(uv --version 2>&1)
echo "[OK] Found uv $UV_VERSION"

# --- Run the tool ---
echo ""
echo "Starting GPU Capacity Finder..."
echo ""

uv run --with "boto3[crt]" --with textual --with rich "$PYTHON_CMD" gpu_capacity_finder.py
