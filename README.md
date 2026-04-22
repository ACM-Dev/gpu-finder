# AWS GPU Instance Capacity Finder (Go Version)

A high-performance, interactive TUI tool written in Go for checking EC2 GPU instance availability across AWS regions. This tool performs **real** On-Demand Capacity Reservation (ODCR) tests and **Capacity Block** availability checks to confirm actual capacity beyond simple dry-runs.

Made by [acuitmeshdev](https://acuitmesh.com).

## Key Features

- **High-Performance Scanning**: Leverages Go goroutines for massive parallel capacity checks.
- **Real ODCR Verification**: Performs actual capacity reservations (and immediate cancellations) to verify real-world availability.
- **Capacity Block (CB) Pricing**: Checks CB offerings across multiple durations (1w, 2w, 4w, 8w, 20w) with retry logic.
- **On-Demand Pricing**: Fetches hourly pricing for confirmed instances only.
- **Polished TUI**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss) for a modern, responsive terminal experience.
- **Dynamic Instance Discovery**: Automatically finds the latest GPU and Accelerator instance types (`p5`, `g6`, `trn1`, etc.) from the AWS API.
- **Export Reports**: Save results to Markdown, JSON, or styled HTML.
- **Region Awareness**: Detects accessible regions vs. those blocked by SCPs or opt-in requirements.

---

## Installation

### Prerequisites

- **AWS Credentials**: Configured via `aws configure`, environment variables, or IAM roles.
- **Permissions**: Requires `ec2:Describe*`, `ec2:CreateCapacityReservation`, `ec2:CancelCapacityReservation`, `ec2:DescribeCapacityBlockOfferings`, `pricing:GetProducts`, and `sts:GetCallerIdentity`.

### Option 1: Download Pre-built Binary

Download the latest release from [Releases](https://github.com/ACM-Dev/gpu-finder/releases):

```bash
# Linux (amd64)
curl -LO https://github.com/ACM-Dev/gpu-finder/releases/latest/download/gpu-finder-vX.Y.Z-linux-amd64.tar.gz
tar xzf gpu-finder-vX.Y.Z-linux-amd64.tar.gz
chmod +x gpu-finder
./gpu-finder

# macOS (arm64 / Apple Silicon)
curl -LO https://github.com/ACM-Dev/gpu-finder/releases/latest/download/gpu-finder-vX.Y.Z-darwin-arm64.tar.gz
tar xzf gpu-finder-vX.Y.Z-darwin-arm64.tar.gz
chmod +x gpu-finder
./gpu-finder

# macOS (amd64 / Intel)
curl -LO https://github.com/ACM-Dev/gpu-finder/releases/latest/download/gpu-finder-vX.Y.Z-darwin-amd64.tar.gz
tar xzf gpu-finder-vX.Y.Z-darwin-amd64.tar.gz
chmod +x gpu-finder
./gpu-finder

# Windows (amd64)
# Download gpu-finder-vX.Y.Z-windows-amd64.zip from Releases, extract, then run:
.\gpu-finder.exe
```

### Option 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/ACM-Dev/gpu-finder.git
cd gpu-finder

# Build the binary
go build -o gpu-finder .

# Run it
./gpu-finder
```

---

## Usage

### Interactive TUI (Default)

```bash
./gpu-finder
```

### CLI Flags

| Flag | Description |
|---|---|
| `--auth` | Check AWS auth, display account details, and exit |
| `--headless` | Skip TUI, run scan interactively, print results |
| `--all` | Scan all accessible regions with P/G series, save all formats |

```bash
# Check authentication
./gpu-finder --auth

# Headless mode (prompts for save formats after scan)
./gpu-finder --headless

# Full auto-scan with all formats saved
./gpu-finder --all
```

---

## Usage Walkthrough

### 1. Welcome & Policy
Review the **Terms of Use**. This tool performs real billing actions (briefly) to ensure results are 100% accurate. Press Enter to accept and continue.

### 2. Region Selection
Select the AWS regions you want to scan.
- `*` marks your default region from AWS config.
- `Space` to toggle, `a` to select all, `n` to select none.
- `Enter` to confirm.

### 3. Instance Selection
Choose the GPU instance types to check. Modern types like `p5.48xlarge` and `g6.48xlarge` are pre-selected by default.
- `Space` to toggle, `a` to select all, `n` to select none.
- `Enter` to start the scan.

### 4. Live Capacity Scan
Watch the live progress bar and spinner as workers fan out across regions and AZs to perform ODCR + CB checks.

### 5. Results Table
Review the final status in a sortable table with GPU specs, ODCR status, and CB pricing.

---

## Navigation Keys

| Key | Action |
|---|---|
| `↑` / `↓` or `j` / `k` | Navigate lists and tables |
| `Space` | Toggle selection |
| `a` | Select All |
| `n` | Select None |
| `Enter` | Proceed to next step / Select row |
| `f` | Toggle ODCR-only filter |
| `d` | Toggle detail panel (CB pricing breakdown) |
| `s` | Save report (choose format: md/json/html/all) |
| `q` | Quit (with confirmation) |
| `Ctrl+C` | Force quit |

---

## Why Go?

This version is a significant upgrade from the original Python implementation:
1. **Speed**: Concurrent checks are handled via lightweight goroutines, making it significantly faster than the `boto3` equivalent.
2. **Zero Dependencies**: Compiles to a single static binary. No need for `pip install` or virtual environments.
3. **Robustness**: Static typing and built-in error handling ensure a stable experience even with complex AWS API interactions.
4. **Full Feature Parity**: Includes Capacity Block checks, on-demand pricing, and multi-format exports.

---

## Disclaimer

**WARNING**: This tool performs **REAL** capacity reservation attempts. While each successful reservation is immediately cancelled, it may briefly incur billing costs or impact your service quotas. Use at your own risk.

---

Made by [acuitmesh](https://acuitmesh.com)'s Dev Team. For issues or contributions, please open a GitHub issue or pull request.
