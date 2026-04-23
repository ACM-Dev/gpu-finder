# AWS GPU Instance Capacity Finder

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

### Option 1: Auto-Install Script (Recommended)

The install script auto-detects your OS/architecture, checks for existing installations, and downloads only if needed.

**Linux / macOS:**
```bash
curl -fsSL https://github.com/ACM-Dev/gpu-finder/raw/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://github.com/ACM-Dev/gpu-finder/raw/main/scripts/install.ps1 | iex
```

### Option 2: Manual Download

Download from [Releases](https://github.com/ACM-Dev/gpu-finder/releases/tag/v1.0.0):

```bash
# Linux (amd64)
curl -LO https://github.com/ACM-Dev/gpu-finder/releases/download/v1.0.0/gpu-finder-v1.0.0-linux-amd64.tar.gz
tar xzf gpu-finder-v1.0.0-linux-amd64.tar.gz
chmod +x gpu-finder
./gpu-finder

# Linux (arm64)
curl -LO https://github.com/ACM-Dev/gpu-finder/releases/download/v1.0.0/gpu-finder-v1.0.0-linux-arm64.tar.gz
tar xzf gpu-finder-v1.0.0-linux-arm64.tar.gz
chmod +x gpu-finder
./gpu-finder

# macOS (arm64 / Apple Silicon)
curl -LO https://github.com/ACM-Dev/gpu-finder/releases/download/v1.0.0/gpu-finder-v1.0.0-darwin-arm64.tar.gz
tar xzf gpu-finder-v1.0.0-darwin-arm64.tar.gz
chmod +x gpu-finder
./gpu-finder

# macOS (amd64 / Intel)
curl -LO https://github.com/ACM-Dev/gpu-finder/releases/download/v1.0.0/gpu-finder-v1.0.0-darwin-amd64.tar.gz
tar xzf gpu-finder-v1.0.0-darwin-amd64.tar.gz
chmod +x gpu-finder
./gpu-finder

# Windows (amd64)
curl -LO https://github.com/ACM-Dev/gpu-finder/releases/download/v1.0.0/gpu-finder-v1.0.0-windows-amd64.tar.gz
tar xzf gpu-finder-v1.0.0-windows-amd64.tar.gz
.\gpu-finder.exe
```

### Option 3: Build from Source

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

## Disclaimer

**WARNING**: This tool performs **REAL** capacity reservation attempts. While each successful reservation is immediately cancelled, it may briefly incur billing costs or impact your service quotas. Use at your own risk.

---

Made by [acuitmesh](https://acuitmesh.com)'s Dev Team. For issues or contributions, please open a GitHub issue or pull request.
