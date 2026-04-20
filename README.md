# AWS GPU Instance Capacity Finder

Interactive tool for checking EC2 GPU instance availability across AWS regions. Performs real ODCR reserve-and-cancel tests to confirm actual capacity, and queries Capacity Block pricing — then displays results in a Textual TUI with export to Markdown, JSON, and HTML.

---

## What it checks

For each selected region and instance type, the tool:

1. **Region access** — detects which regions are accessible vs. blocked by SCP
2. **Instance availability** — which AZs offer the instance type
3. **ODCR (On-Demand Capacity Reservation)** — performs a real reservation then immediately cancels it to confirm actual capacity (not just a dry-run)
4. **Capacity Block offerings** — queries available fixed-window reservations with pricing at 1w / 2w / 4w / 8w durations

### Instance families covered

| Family | Examples | Accelerator |
|---|---|---|
| P-series | p3, p3dn, p4d, p4de, p5, p5en, p5e | NVIDIA V100 / A100 / H100 / H200 |
| G-series | g5, g6, g6e | NVIDIA A10G / L4 / L40S |
| AWS Silicon | trn1, inf2 | Trainium / Inferentia2 |

GPU specs (count, VRAM, vCPUs) are fetched live from the AWS API — not hardcoded.

### Regions checked

| Region | Location |
|---|---|
| ap-southeast-1 | Singapore |
| ap-southeast-3 | Jakarta |
| ap-southeast-7 | Bangkok |
| ap-northeast-1 | Tokyo |
| ap-northeast-2 | Seoul |
| ap-south-1 | Mumbai |
| ap-southeast-2 | Sydney |
| us-east-1 | N. Virginia |
| us-east-2 | Ohio |
| us-west-2 | Oregon |

---

## Requirements

- Python 3.11+
- AWS credentials configured (`aws configure`, `AWS_PROFILE`, or env vars)
- IAM permissions: `ec2:Describe*`, `ec2:CreateCapacityReservation`, `ec2:CancelCapacityReservation`, `sts:GetCallerIdentity`, `organizations:DescribeOrganization` (optional)

---

## Running locally

### With uv (recommended — no install needed)

```bash
# Full interactive TUI
uv run --with "boto3[crt]" --with textual --with rich python3 gpu_capacity_finder.py

# Check auth and exit
uv run --with "boto3[crt]" --with textual --with rich python3 gpu_capacity_finder.py --check-auth

# Non-interactive — scan all accessible regions, save all output formats
uv run --with "boto3[crt]" --with textual --with rich python3 gpu_capacity_finder.py --no-tui --all
```

### With pip

```bash
pip install "boto3[crt]" textual rich
python3 gpu_capacity_finder.py
```

---

## Running with Docker

### Pull from registry

```bash
docker pull ghcr.io/acm-dev/aws-gpu-instance-finder:latest
```

### Run with environment variables

```bash
docker run -it --rm \
  -e AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY \
  -e AWS_SESSION_TOKEN \
  -v $(pwd)/output:/app/output \
  ghcr.io/acm-dev/aws-gpu-instance-finder:latest
```

### Run with AWS profile

```bash
docker run -it --rm \
  -v ~/.aws:/root/.aws:ro \
  -e AWS_PROFILE=your-profile \
  -v $(pwd)/output:/app/output \
  ghcr.io/acm-dev/aws-gpu-instance-finder:latest
```

### Build locally

```bash
docker build -t gpu-capacity-finder .
docker run -it --rm \
  -e AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY \
  -e AWS_SESSION_TOKEN \
  -v $(pwd)/output:/app/output \
  gpu-capacity-finder
```

Output files are written to `/app/output/` inside the container — mount a host directory to persist them.

---

## Usage walkthrough

### Step 1 — Auth check
The tool prints your account ID, ARN, and org details, then exits if `--check-auth` is passed.

### Step 2 — Region selection
Regions are tested for SCP access. Accessible regions are shown with `✓`, blocked ones with `✗`. Select by entering comma-separated numbers, or press Enter to use all accessible regions.

```
  1.  ap-southeast-1        Singapore            [✓ Accessible]
  2.  ap-southeast-3        Jakarta              [✓ Accessible]
  3.  ap-northeast-1        Tokyo                [✓ Accessible]
  4.  ap-northeast-2        Seoul                [✗ SCP Blocked]
  ...
```

### Step 3 — Instance selection
Instance types are listed with live GPU specs from the AWS API. Select by number or press Enter for all P-series (default).

```
   1. [P] p3.16xlarge        8x V100 16GB (128GB total) 64vCPU
   2. [P] p4d.24xlarge       8x A100 40GB (320GB total) 96vCPU
   3. [P] p5.48xlarge        8x H100 80GB (640GB total) 192vCPU
   4. [P] p5en.48xlarge      8x H200 141GB (1128GB total) 192vCPU
   ...
```

### Step 4 — Scan
Runs in parallel across all region/instance/AZ combinations. Shows a progress bar. ODCR checks perform a real reservation then immediately cancel it — so results reflect actual capacity, not just API dry-run responses.

### Step 5 — TUI report

| Key | Action |
|---|---|
| `↑` `↓` | Navigate rows |
| `Enter` | Expand Capacity Block pricing for selected row |
| `f` | Toggle filter — show ODCR-confirmed rows only |
| `s` | Save report (choose MD / JSON / HTML / all) |
| `q` | Quit |

---

## Output formats

Reports are saved as `gpu-capacity-YYYY-MM-DD.{ext}` in the same directory as the script (or `/app/output/` in Docker).

| Format | Contents |
|---|---|
| `.md` | Markdown tables — compatible with `convert_to_docx.py` for Word export |
| `.json` | Structured data with GPU specs, ODCR status, and full CB pricing per AZ |
| `.html` | Self-contained browser report with styled tables |

---

## ODCR status codes

| Status | Meaning |
|---|---|
| `Confirmed` | Real reservation succeeded and was immediately cancelled — capacity confirmed |
| `InsufficientInstanceCapacity` | No stock available in this AZ |
| `Unsupported` | Instance type does not support ODCR |
| `InstanceLimitExceeded (quota N vCPU)` | Account vCPU quota too low — capacity may exist but quota increase needed |
| `Error` | Unexpected API error |

---

## CI/CD

The Docker image is built and pushed automatically via GitHub Actions on every push to `main`. The workflow uses a self-hosted runner and pushes to `ghcr.io/acm-dev/aws-gpu-instance-finder`.

Tags published:
- `latest` — latest main branch build
- `sha-<short>` — pinned to a specific commit
- branch name / PR number for non-main builds
