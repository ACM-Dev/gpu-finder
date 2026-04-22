"""
GPU Capacity Finder — interactive TUI for finding EC2 GPU instance capacity.

Usage:
    uv run --with "boto3[crt]" --with textual --with rich python3 gpu_capacity_finder.py
    uv run --with "boto3[crt]" --with textual --with rich python3 gpu_capacity_finder.py --check-auth
    uv run --with "boto3[crt]" --with textual --with rich python3 gpu_capacity_finder.py --no-tui --all
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from datetime import date, datetime, timedelta
from typing import Optional

import boto3
from botocore.exceptions import ClientError, NoCredentialsError, BotoCoreError

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

CANDIDATE_REGIONS = [
    ("ap-southeast-1", "Singapore"),
    ("ap-southeast-3", "Jakarta"),
    ("ap-southeast-7", "Bangkok"),
    ("ap-northeast-1", "Tokyo"),
    ("ap-northeast-2", "Seoul"),
    ("ap-south-1", "Mumbai"),
    ("ap-southeast-2", "Sydney"),
    ("us-east-1", "N. Virginia"),
    ("us-east-2", "Ohio"),
    ("us-west-2", "Oregon"),
    ("us-west-1", "N. California")
]

# Instances that may not exist in all regions — queried per-region
REGION_SPECIFIC = {"p5e.48xlarge"}

# Default P-series instances shown first in selection
P_SERIES = [
    "p3.16xlarge", "p3dn.24xlarge", "p4d.24xlarge", "p4de.24xlarge",
    "p5.48xlarge", "p5en.48xlarge", "p5e.48xlarge",
]
G_SERIES = ["g5.48xlarge", "g6.48xlarge", "g6e.48xlarge"]
AWS_SILICON = ["trn1.32xlarge", "inf2.48xlarge"]

ALL_CANDIDATES = P_SERIES + G_SERIES + AWS_SILICON

CB_DURATIONS_HOURS = [168, 336, 672, 1344, 3360]  # 1w, 2w, 4w, 8w, 20w

# Pricing API location strings (only available via us-east-1 endpoint)
REGION_TO_PRICING_LOCATION = {
    "us-east-1":      "US East (N. Virginia)",
    "us-east-2":      "US East (Ohio)",
    "us-west-1":      "US West (N. California)",
    "us-west-2":      "US West (Oregon)",
    "ap-southeast-1": "Asia Pacific (Singapore)",
    "ap-southeast-2": "Asia Pacific (Sydney)",
    "ap-southeast-3": "Asia Pacific (Jakarta)",
    "ap-southeast-7": "Asia Pacific (Thailand)",
    "ap-northeast-1": "Asia Pacific (Tokyo)",
    "ap-northeast-2": "Asia Pacific (Seoul)",
    "ap-south-1":     "Asia Pacific (Mumbai)",
}

# ---------------------------------------------------------------------------
# Data classes
# ---------------------------------------------------------------------------

@dataclass
class GpuSpec:
    instance_type: str
    gpu_count: int
    gpu_name: str
    gpu_mfr: str
    per_gpu_mib: int
    total_gpu_mib: int
    vcpus: int

    @property
    def summary(self) -> str:
        per_gb = self.per_gpu_mib / 1024
        total_gb = self.total_gpu_mib / 1024
        return f"{self.gpu_count}x {self.gpu_name} {per_gb:.0f}GB ({total_gb:.0f}GB total) {self.vcpus}vCPU"


@dataclass
class CbOffering:
    duration_hours: int
    start_date: str
    end_date: str
    upfront_fee: float
    az: str


@dataclass
class CapacityResult:
    region: str
    instance_type: str
    az: str
    odcr_status: str
    odcr_detail: str = ""
    cb_offerings: list[CbOffering] = field(default_factory=list)
    cb_error: str = ""
    ondemand_price_hr: Optional[float] = None


# Fallback GPU specs when API fetch fails
GPU_SPECS_FALLBACK = {
    "p3.16xlarge":  GpuSpec("p3.16xlarge", 8, "V100",   "NVIDIA", 16384, 131072, 64),
    "p3dn.24xlarge": GpuSpec("p3dn.24xlarge", 8, "V100",  "NVIDIA", 16384, 131072, 96),
    "p4d.24xlarge": GpuSpec("p4d.24xlarge", 8, "A100",   "NVIDIA", 40960, 327680, 96),
    "p4de.24xlarge": GpuSpec("p4de.24xlarge", 8, "A100",  "NVIDIA", 81920, 655360, 96),
    "p5.48xlarge":  GpuSpec("p5.48xlarge", 8, "H100",    "NVIDIA", 81920, 655360, 192),
    "p5en.48xlarge": GpuSpec("p5en.48xlarge", 8, "H200",  "NVIDIA", 144896, 1159168, 192),
    "p5e.48xlarge": GpuSpec("p5e.48xlarge", 8, "H200",   "NVIDIA", 144896, 1159168, 192),
    "g5.48xlarge":  GpuSpec("g5.48xlarge", 8, "A10G",    "NVIDIA", 24576, 196608, 192),
    "g6.48xlarge":  GpuSpec("g6.48xlarge", 8, "L4",      "NVIDIA", 24576, 196608, 192),
    "g6e.48xlarge": GpuSpec("g6e.48xlarge", 8, "L40S",   "NVIDIA", 49152, 393216, 192),
    "trn1.32xlarge": GpuSpec("trn1.32xlarge", 16, "Trainium", "AWS", 32768, 524288, 128),
    "inf2.48xlarge": GpuSpec("inf2.48xlarge", 12, "Inferentia2", "AWS", 32768, 393216, 192),
}


# ---------------------------------------------------------------------------
# Auth check
# ---------------------------------------------------------------------------

def check_auth() -> dict:
    try:
        sts = boto3.client("sts")
        identity = sts.get_caller_identity()
    except (NoCredentialsError, BotoCoreError) as e:
        print(f"ERROR: No AWS credentials found — {e}")
        print("Please login with `aws login` or `aws configure`, or set credentials in environment variables.")
        print("Set AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN, or use AWS_PROFILE.")
        sys.exit(1)
    except ClientError as e:
        print(f"ERROR: Auth failed — {e}")
        sys.exit(1)

    print(f"Account : {identity['Account']}")
    print(f"ARN     : {identity['Arn']}")

    try:
        org = boto3.client("organizations").describe_organization()["Organization"]
        print(f"Org     : {org['Id']}  (master: {org['MasterAccountId']} / {org['MasterAccountEmail']})")
    except ClientError:
        print("Org     : (no organizations access)")

    return identity


# ---------------------------------------------------------------------------
# Region access check
# ---------------------------------------------------------------------------

def check_region_access(region: str) -> bool:
    try:
        boto3.client("ec2", region_name=region).describe_availability_zones(
            Filters=[{"Name": "state", "Values": ["available"]}]
        )
        return True
    except ClientError as e:
        code = e.response["Error"]["Code"]
        if code in ("AuthFailure", "UnauthorizedOperation", "AccessDeniedException",
                    "OptInRequired", "InvalidClientTokenId"):
            return False
        return False
    except Exception:
        return False


# ---------------------------------------------------------------------------
# GPU spec lookup
# ---------------------------------------------------------------------------

def fetch_gpu_specs(instance_types: list[str], region: str = "us-east-1") -> dict[str, GpuSpec]:
    specs: dict[str, GpuSpec] = {}
    # Try the provided region first, then fall back to us-east-1 which has the broadest instance coverage
    regions_to_try = list(dict.fromkeys([region, "us-east-1", "ap-northeast-1"]))
    to_fetch = [t for t in instance_types if t not in REGION_SPECIFIC]

    for r in regions_to_try:
        remaining = [t for t in to_fetch if t not in specs]
        if not remaining:
            break
        ec2 = boto3.client("ec2", region_name=r)
        # Fetch one at a time — batch calls fail entirely if any single type doesn't exist in the region
        for itype in remaining:
            try:
                resp = ec2.describe_instance_types(InstanceTypes=[itype])
                for it in resp["InstanceTypes"]:
                    gpu_info = it.get("GpuInfo", {})
                    gpus = gpu_info.get("Gpus", [{}])
                    gpu = gpus[0] if gpus else {}
                    specs[itype] = GpuSpec(
                        instance_type=itype,
                        gpu_count=gpu.get("Count", 0),
                        gpu_name=gpu.get("Name", "N/A"),
                        gpu_mfr=gpu.get("Manufacturer", "N/A"),
                        per_gpu_mib=gpu.get("MemoryInfo", {}).get("SizeInMiB", 0),
                        total_gpu_mib=gpu_info.get("TotalGpuMemoryInMiB", 0),
                        vcpus=it.get("VCpuInfo", {}).get("DefaultVCpus", 0),
                    )
            except ClientError:
                pass  # type doesn't exist in this region, try next region

    # Always fill remaining gaps from hardcoded fallback (confirmed from AWS API)
    for t in instance_types:
        if t not in specs and t in GPU_SPECS_FALLBACK:
            specs[t] = GPU_SPECS_FALLBACK[t]
    return specs


def fetch_gpu_specs_for_region(instance_types: list[str], region: str) -> dict[str, GpuSpec]:
    region_specific = [t for t in instance_types if t in REGION_SPECIFIC]
    if not region_specific:
        return {}
    ec2 = boto3.client("ec2", region_name=region)
    specs: dict[str, GpuSpec] = {}
    try:
        resp = ec2.describe_instance_types(InstanceTypes=region_specific)
        for it in resp["InstanceTypes"]:
            itype = it["InstanceType"]
            gpu_info = it.get("GpuInfo", {})
            gpus = gpu_info.get("Gpus", [{}])
            gpu = gpus[0] if gpus else {}
            specs[itype] = GpuSpec(
                instance_type=itype,
                gpu_count=gpu.get("Count", 0),
                gpu_name=gpu.get("Name", "N/A"),
                gpu_mfr=gpu.get("Manufacturer", "N/A"),
                per_gpu_mib=gpu.get("MemoryInfo", {}).get("SizeInMiB", 0),
                total_gpu_mib=gpu_info.get("TotalGpuMemoryInMiB", 0),
                vcpus=it.get("VCpuInfo", {}).get("DefaultVCpus", 0),
            )
    except ClientError:
        pass
    for t in region_specific:
        if t not in specs and t in GPU_SPECS_FALLBACK:
            specs[t] = GPU_SPECS_FALLBACK[t]
    return specs


# ---------------------------------------------------------------------------
# On-demand pricing
# ---------------------------------------------------------------------------

def fetch_ondemand_prices(instance_types: list[str], regions: list[str]) -> dict[tuple[str, str], float]:
    """Returns {(region, instance_type): price_per_hour}. Uses us-east-1 pricing endpoint."""
    prices: dict[tuple[str, str], float] = {}
    try:
        pricing = boto3.client("pricing", region_name="us-east-1")
    except Exception:
        return prices

    for region in regions:
        location = REGION_TO_PRICING_LOCATION.get(region)
        if not location:
            continue
        for itype in instance_types:
            try:
                resp = pricing.get_products(
                    ServiceCode="AmazonEC2",
                    Filters=[
                        {"Type": "TERM_MATCH", "Field": "instanceType",    "Value": itype},
                        {"Type": "TERM_MATCH", "Field": "location",        "Value": location},
                        {"Type": "TERM_MATCH", "Field": "tenancy",         "Value": "Shared"},
                        {"Type": "TERM_MATCH", "Field": "operatingSystem", "Value": "Linux"},
                        {"Type": "TERM_MATCH", "Field": "capacitystatus",  "Value": "Used"},
                        {"Type": "TERM_MATCH", "Field": "preInstalledSw",  "Value": "NA"},
                    ],
                    MaxResults=1,
                )
                if not resp["PriceList"]:
                    continue
                product = json.loads(resp["PriceList"][0])
                for term in product["terms"]["OnDemand"].values():
                    for dim in term["priceDimensions"].values():
                        price = float(dim["pricePerUnit"].get("USD", 0))
                        if price > 0:
                            prices[(region, itype)] = price
            except (ClientError, KeyError, json.JSONDecodeError):
                pass

    return prices


# ---------------------------------------------------------------------------
# Capacity scanning
# ---------------------------------------------------------------------------

def get_available_azs(ec2, instance_type: str) -> list[str]:
    try:
        resp = ec2.describe_instance_type_offerings(
            LocationType="availability-zone",
            Filters=[{"Name": "instance-type", "Values": [instance_type]}],
        )
        return [o["Location"] for o in resp.get("InstanceTypeOfferings", [])]
    except ClientError:
        return []


def check_odcr(ec2, instance_type: str, az: str) -> tuple[str, str]:
    """Returns (status, detail). Does real reserve+cancel if dry-run succeeds."""
    try:
        ec2.create_capacity_reservation(
            InstanceType=instance_type,
            InstancePlatform="Linux/UNIX",
            AvailabilityZone=az,
            InstanceCount=1,
            InstanceMatchCriteria="targeted",
            DryRun=True,
        )
        status = "dry-run-ok"
    except ClientError as e:
        code = e.response["Error"]["Code"]
        msg = e.response["Error"].get("Message", "")
        if code == "DryRunOperation":
            status = "dry-run-ok"
        elif code == "InsufficientInstanceCapacity":
            return "InsufficientInstanceCapacity", ""
        elif code in ("UnsupportedOperation", "Unsupported"):
            return "Unsupported", ""
        elif code == "InstanceLimitExceeded":
            vcpu_limit = ""
            import re
            m = re.search(r"vCPU limit of (\d+)", msg)
            if m:
                vcpu_limit = f" (quota {m.group(1)} vCPU)"
            return f"InstanceLimitExceeded{vcpu_limit}", msg
        else:
            return f"Error", f"{code}: {msg}"

    # Dry-run succeeded — do real reservation and immediately cancel
    cr_id = None
    try:
        resp = ec2.create_capacity_reservation(
            InstanceType=instance_type,
            InstancePlatform="Linux/UNIX",
            AvailabilityZone=az,
            InstanceCount=1,
            InstanceMatchCriteria="targeted",
        )
        cr_id = resp["CapacityReservation"]["CapacityReservationId"]
        return "Confirmed", cr_id
    except ClientError as e:
        code = e.response["Error"]["Code"]
        msg = e.response["Error"].get("Message", "")
        if code == "InsufficientInstanceCapacity":
            return "InsufficientInstanceCapacity", ""
        return "Error", f"{code}: {msg}"
    finally:
        if cr_id:
            try:
                ec2.cancel_capacity_reservation(CapacityReservationId=cr_id)
            except ClientError:
                pass  # best-effort cancel


def check_capacity_blocks(ec2, instance_type: str) -> tuple[list[CbOffering], str]:
    """Query CB offerings for the whole region (not per-AZ) — CB AZ is determined by AWS."""
    offerings: list[CbOffering] = []
    now = datetime.utcnow()
    now_str = now.strftime("%Y-%m-%dT%H:%M:%SZ")

    for hours in CB_DURATIONS_HOURS:
        # EndDateRange must be far enough ahead that a block of this duration can fit within 20 weeks.
        # Use 20 weeks minus the block duration as the latest possible start, so the block ends within window.
        max_start = now + timedelta(weeks=20) - timedelta(hours=hours)
        if max_start <= now:
            continue  # block too long to fit in 20-week window
        end_range = max_start.strftime("%Y-%m-%dT%H:%M:%SZ")

        for attempt in range(4):  # retry up to 4 times on throttle
            try:
                resp = ec2.describe_capacity_block_offerings(
                    InstanceType=instance_type,
                    InstanceCount=1,
                    CapacityDurationHours=hours,
                    StartDateRange=now_str,
                    EndDateRange=end_range,
                )
                all_offers = resp.get("CapacityBlockOfferings", [])
                if all_offers:
                    earliest = sorted(all_offers, key=lambda x: x["StartDate"])[0]
                    offerings.append(CbOffering(
                        duration_hours=hours,
                        start_date=str(earliest["StartDate"])[:10],
                        end_date=str(earliest["EndDate"])[:10],
                        upfront_fee=float(earliest["UpfrontFee"]),  # API returns string e.g. "1982.4000"
                        az=earliest["AvailabilityZone"],
                    ))
                break  # success
            except ClientError as e:
                code = e.response["Error"]["Code"]
                msg = e.response["Error"].get("Message", "")
                if code == "RequestLimitExceeded" and attempt < 3:
                    time.sleep(2 ** attempt)  # 1s, 2s, 4s backoff
                    continue
                if code in ("InvalidAction", "UnsupportedOperation"):
                    return [], "Not supported in this region"
                if code == "PendingVerification":
                    return [], "Org master account pending verification"
                if code == "InvalidParameterValue":
                    if "not supported for Capacity Blocks" in msg:
                        return [], f"CB not supported: {msg}"
                    # Duration or date issue — skip just this duration
                    break
                return [], f"{code}: {msg}"
            except Exception as e:
                break  # skip this duration on unexpected error

    return offerings, ""


def scan_region_instance(region: str, instance_type: str) -> list[CapacityResult]:
    results = []
    ec2 = boto3.client("ec2", region_name=region)

    azs = get_available_azs(ec2, instance_type)
    if not azs:
        return results

    # Query CB once per region/instance — not per AZ (AWS determines which AZ has CB capacity)
    cb_offerings, cb_error = check_capacity_blocks(ec2, instance_type)

    for az in sorted(azs):
        odcr_status, odcr_detail = check_odcr(ec2, instance_type, az)
        results.append(CapacityResult(
            region=region,
            instance_type=instance_type,
            az=az,
            odcr_status=odcr_status,
            odcr_detail=odcr_detail,
            cb_offerings=cb_offerings,
            cb_error=cb_error,
        ))

    return results


# ---------------------------------------------------------------------------
# Export
# ---------------------------------------------------------------------------

def export_markdown(results: list[CapacityResult], gpu_specs: dict[str, GpuSpec], account_id: str) -> str:
    today = date.today().strftime("%B %d, %Y")
    lines = [
        f"# GPU Capacity Research",
        f"",
        f"**Date:** {today}",
        f"**Account:** {account_id}",
        f"",
        f"---",
        f"",
        f"## ODCR Results",
        f"",
        f"| Instance | GPUs | Region | AZ | ODCR Status |",
        f"|---|---|---|---|---|",
    ]
    for r in results:
        spec = gpu_specs.get(r.instance_type)
        gpu_str = spec.summary if spec else "N/A"
        status = r.odcr_status
        if r.odcr_detail and "Error" in status:
            status = f"{status} — {r.odcr_detail}"
        lines.append(f"| `{r.instance_type}` | {gpu_str} | {r.region} | {r.az.split('-')[-1]} | {status} |")

    lines += ["", "---", "", "## Capacity Block Offerings", ""]
    cb_results = [r for r in results if r.cb_offerings]
    if cb_results:
        lines += [
            "| Instance | GPUs | Region | AZ | Duration | Start | End | Upfront | /month |",
            "|---|---|---|---|---|---|---|---|---|",
        ]
        for r in cb_results:
            spec = gpu_specs.get(r.instance_type)
            gpu_str = spec.summary if spec else "N/A"
            for cb in r.cb_offerings:
                weeks = cb.duration_hours // 168
                monthly = int(cb.upfront_fee / (cb.duration_hours / 24 / 30.44))
                lines.append(
                    f"| `{r.instance_type}` | {gpu_str} | {r.region} | {cb.az.split('-')[-1]} "
                    f"| {weeks}w | {cb.start_date} | {cb.end_date} | ${int(cb.upfront_fee):,} | ~${monthly:,}/mo |"
                )
    else:
        lines.append("> No Capacity Block offerings found.")

    return "\n".join(lines)


def export_json(results: list[CapacityResult], gpu_specs: dict[str, GpuSpec], account_id: str) -> str:
    out: dict = {"account": account_id, "date": date.today().isoformat(), "results": []}
    for r in results:
        spec = gpu_specs.get(r.instance_type)
        out["results"].append({
            "region": r.region,
            "instance_type": r.instance_type,
            "az": r.az,
            "gpu_spec": {
                "count": spec.gpu_count if spec else None,
                "name": spec.gpu_name if spec else None,
                "per_gpu_gb": round(spec.per_gpu_mib / 1024, 1) if spec else None,
                "total_gb": round(spec.total_gpu_mib / 1024, 1) if spec else None,
                "vcpus": spec.vcpus if spec else None,
            },
            "odcr_status": r.odcr_status,
            "odcr_detail": r.odcr_detail,
            "capacity_blocks": [
                {
                    "duration_hours": cb.duration_hours,
                    "duration_weeks": cb.duration_hours // 168,
                    "start_date": cb.start_date,
                    "end_date": cb.end_date,
                    "upfront_fee_usd": cb.upfront_fee,
                    "az": cb.az,
                }
                for cb in r.cb_offerings
            ],
            "cb_error": r.cb_error,
        })
    return json.dumps(out, indent=2)


def export_html(results: list[CapacityResult], gpu_specs: dict[str, GpuSpec], account_id: str) -> str:
    today = date.today().strftime("%B %d, %Y")

    def status_color(s: str) -> str:
        if "Confirmed" in s:
            return "#1a7a3c"
        if "Insufficient" in s:
            return "#b91c1c"
        if "Unsupported" in s:
            return "#6b7280"
        if "Limit" in s or "quota" in s.lower():
            return "#b45309"
        return "#374151"

    rows = ""
    for r in results:
        spec = gpu_specs.get(r.instance_type)
        gpu_str = spec.summary if spec else "N/A"
        cb1 = next((cb for cb in r.cb_offerings if cb.duration_hours == 168), None)
        cb_str = f"{cb1.start_date} / ${int(cb1.upfront_fee):,}/1w" if cb1 else (r.cb_error or "—")
        color = status_color(r.odcr_status)
        status_text = r.odcr_status
        if r.odcr_detail and "Error" in r.odcr_status:
            status_text = f"{r.odcr_status}<br><small>{r.odcr_detail}</small>"
        rows += (
            f"<tr>"
            f"<td><code>{r.instance_type}</code></td>"
            f"<td>{gpu_str}</td>"
            f"<td>{r.region}</td>"
            f"<td>{r.az.split('-')[-1]}</td>"
            f"<td style='color:{color};font-weight:600'>{status_text}</td>"
            f"<td>{cb_str}</td>"
            f"</tr>\n"
        )

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>GPU Capacity Report — {today}</title>
<style>
  body {{ font-family: Calibri, sans-serif; background: #f8fafc; color: #1a1a1a; margin: 40px; }}
  h1 {{ color: #1F4E79; border-bottom: 3px solid #1F4E79; padding-bottom: 8px; }}
  h2 {{ color: #1F4E79; margin-top: 32px; }}
  table {{ border-collapse: collapse; width: 100%; margin-top: 12px; }}
  th {{ background: #1F4E79; color: #fff; padding: 8px 12px; text-align: left; font-size: 13px; }}
  td {{ padding: 7px 12px; font-size: 13px; border-bottom: 1px solid #e5e7eb; }}
  tr:nth-child(even) td {{ background: #F0F4FA; }}
  code {{ background: #EEF3F8; padding: 2px 6px; border-radius: 3px; color: #1F4E79; font-size: 12px; }}
  .meta {{ color: #4472C4; font-size: 14px; margin-bottom: 24px; }}
</style>
</head>
<body>
<h1>GPU Capacity Research</h1>
<div class="meta"><strong>Date:</strong> {today} &nbsp;|&nbsp; <strong>Account:</strong> {account_id}</div>
<h2>ODCR &amp; Capacity Block Results</h2>
<table>
<tr>
  <th>Instance</th><th>GPUs</th><th>Region</th><th>AZ</th><th>ODCR Status</th><th>CB Earliest Start / 1w Price</th>
</tr>
{rows}
</table>
</body>
</html>"""


def save_outputs(results: list[CapacityResult], gpu_specs: dict[str, GpuSpec],
                 account_id: str, formats: list[str], out_dir: str = ".") -> list[str]:
    today = date.today().strftime("%Y-%m-%d")
    saved = []
    for fmt in formats:
        path = os.path.join(out_dir, f"gpu-capacity-{today}.{fmt}")
        if fmt == "md":
            content = export_markdown(results, gpu_specs, account_id)
        elif fmt == "json":
            content = export_json(results, gpu_specs, account_id)
        elif fmt == "html":
            content = export_html(results, gpu_specs, account_id)
        else:
            continue
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)
        saved.append(path)
    return saved


# ---------------------------------------------------------------------------
# Interactive CLI selection helpers (plain terminal, used before TUI)
# ---------------------------------------------------------------------------

def select_regions(accessible: list[tuple[str, str, bool]]) -> list[str]:
    print("\nAvailable regions:")
    for i, (code, name, ok) in enumerate(accessible, 1):
        status = "✓ Accessible" if ok else "✗ SCP Blocked"
        print(f"  {i:2}. {code:<22} {name:<20} [{status}]")
    print("\nEnter region numbers (comma-separated) or press Enter for all accessible:")
    raw = input("> ").strip()
    accessible_codes = [code for code, _, ok in accessible if ok]
    if not raw:
        return accessible_codes
    selected = []
    for part in raw.split(","):
        part = part.strip()
        if part.isdigit():
            idx = int(part) - 1
            if 0 <= idx < len(accessible):
                code, _, ok = accessible[idx]
                if ok:
                    selected.append(code)
                else:
                    print(f"  Skipping {code} — SCP blocked")
    return selected or accessible_codes


def select_instances(gpu_specs: dict[str, GpuSpec]) -> list[str]:
    all_types = list({t: None for t in P_SERIES + G_SERIES + AWS_SILICON}.keys())
    print("\nAvailable instance types:")
    for i, itype in enumerate(all_types, 1):
        spec = gpu_specs.get(itype)
        gpu_str = f"  {spec.summary}" if spec else "  (specs fetched per region)"
        marker = "[P]" if itype in P_SERIES else ("[G]" if itype in G_SERIES else "[A]")
        print(f"  {i:2}. {marker} {itype:<22}{gpu_str}")
    print("\nEnter numbers (comma-separated) or Enter for all P-series [default]:")
    raw = input("> ").strip()
    if not raw:
        return P_SERIES
    selected = []
    for part in raw.split(","):
        part = part.strip()
        if part.isdigit():
            idx = int(part) - 1
            if 0 <= idx < len(all_types):
                selected.append(all_types[idx])
    return selected or P_SERIES


# ---------------------------------------------------------------------------
# Scanning with progress
# ---------------------------------------------------------------------------

def run_scan(regions: list[str], instances: list[str], progress_cb=None) -> list[CapacityResult]:
    tasks = [(r, i) for r in regions for i in instances]
    results: list[CapacityResult] = []

    with ThreadPoolExecutor(max_workers=5) as ex:
        futures = {ex.submit(scan_region_instance, r, i): (r, i) for r, i in tasks}
        for fut in as_completed(futures):
            r, i = futures[fut]
            try:
                results.extend(fut.result())
            except Exception as e:
                results.append(CapacityResult(
                    region=r, instance_type=i, az="?",
                    odcr_status="Error", odcr_detail=str(e),
                ))
            if progress_cb:
                progress_cb(r, i)

    return sorted(results, key=lambda x: (x.region, x.instance_type, x.az))


# ---------------------------------------------------------------------------
# Textual TUI
# ---------------------------------------------------------------------------

def run_tui(results: list[CapacityResult], gpu_specs: dict[str, GpuSpec], account_id: str):
    try:
        from textual.app import App, ComposeResult
        from textual.binding import Binding
        from textual.widgets import DataTable, Footer, Header, Label, Select, Static
        from textual.containers import Vertical, Horizontal
        from textual.screen import ModalScreen
        from rich.text import Text
    except ImportError:
        print("textual not installed. Install with: pip install textual")
        print_plain_report(results, gpu_specs)
        return

    ODCR_COLORS = {
        "Confirmed": "bold green",
        "InsufficientInstanceCapacity": "bold red",
        "Unsupported": "dim",
        "Error": "red",
    }

    def odcr_text(status: str) -> Text:
        for key, style in ODCR_COLORS.items():
            if key in status:
                return Text(status, style=style)
        return Text(status, style="yellow")

    class SaveScreen(ModalScreen):
        BINDINGS = [Binding("escape", "dismiss", "Cancel")]

        def __init__(self, results, gpu_specs, account_id):
            super().__init__()
            self._results = results
            self._gpu_specs = gpu_specs
            self._account_id = account_id

        def compose(self) -> ComposeResult:
            yield Vertical(
                Static("Save report — choose formats (enter numbers, comma-separated):\n"
                       "  1. Markdown (.md)\n  2. JSON (.json)\n  3. HTML (.html)\n  4. All\n",
                       id="save-prompt"),
                id="save-box",
            )

        def on_mount(self):
            self.query_one("#save-box").border_title = "Export"

        def on_key(self, event):
            if event.key in ("1", "2", "3", "4", "enter"):
                fmt_map = {"1": ["md"], "2": ["json"], "3": ["html"], "4": ["md", "json", "html"]}
                fmts = fmt_map.get(event.key, ["md", "json", "html"])
                out_dir = os.path.dirname(os.path.abspath(__file__))
                saved = save_outputs(self._results, self._gpu_specs, self._account_id, fmts, out_dir)
                self.app.notify(f"Saved: {', '.join(saved)}", title="Exported", severity="information")
                self.dismiss()

    class GpuFinderApp(App):
        CSS = """
        Screen { background: #0d1117; }
        Header { background: #1F4E79; color: white; }
        Footer { background: #1F4E79; color: white; }
        #filters { height: 3; padding: 0 2; }
        DataTable { height: 1fr; }
        DataTable > .datatable--header { background: #1F4E79; color: white; }
        DataTable > .datatable--cursor { background: #4472C4; color: white; }
        #detail-panel { height: 12; border: solid #4472C4; padding: 1 2; margin: 0 0 1 0; display: none; }
        #detail-panel.visible { display: block; }
        #save-box { width: 50; height: 14; border: solid #4472C4; background: #0d1117;
                    padding: 1 2; align: center middle; }
        """
        BINDINGS = [
            Binding("q", "quit", "Quit"),
            Binding("s", "save", "Save"),
            Binding("f", "toggle_filter", "ODCR only"),
            Binding("enter", "expand", "Detail"),
        ]
        TITLE = "GPU Capacity Finder"

        def __init__(self, results, gpu_specs, account_id):
            super().__init__()
            self._all_results = results
            self._gpu_specs = gpu_specs
            self._account_id = account_id
            self._odcr_filter = False
            self._selected_row: Optional[CapacityResult] = None

        def compose(self) -> ComposeResult:
            yield Header(show_clock=True)
            yield Label(
                f"  Account: {self._account_id}   |   "
                f"[f] toggle ODCR-only filter   |   [enter] show CB pricing   |   [s] save report",
                id="filters",
            )
            yield DataTable(id="main-table", zebra_stripes=True, cursor_type="row")
            yield Static(id="detail-panel")
            yield Footer()

        def on_mount(self):
            self._refresh_table()

        def _visible_results(self) -> list[CapacityResult]:
            if self._odcr_filter:
                return [r for r in self._all_results if "Confirmed" in r.odcr_status]
            return self._all_results

        def _refresh_table(self):
            table = self.query_one("#main-table", DataTable)
            table.clear(columns=True)
            table.add_columns("Instance", "GPUs", "Region", "AZ", "ODCR Status", "CB Start", "CB 1w", "CB 4w")
            for r in self._visible_results():
                spec = self._gpu_specs.get(r.instance_type)
                gpu_str = spec.summary if spec else "—"
                cb1_offer = next((cb for cb in r.cb_offerings if cb.duration_hours == 168), None)
                cb4_offer = next((cb for cb in r.cb_offerings if cb.duration_hours == 672), None)
                cb_start = cb1_offer.start_date if cb1_offer else "—"
                cb1 = f"${int(cb1_offer.upfront_fee):,}" if cb1_offer else "—"
                cb4 = f"${int(cb4_offer.upfront_fee):,}" if cb4_offer else "—"
                status_display = r.odcr_status
                if r.odcr_detail and "Error" in r.odcr_status:
                    status_display = f"{r.odcr_status} — {r.odcr_detail}"
                table.add_row(
                    r.instance_type, gpu_str, r.region,
                    r.az.split("-")[-1],
                    odcr_text(status_display),
                    cb_start, cb1, cb4,
                )

        def action_toggle_filter(self):
            self._odcr_filter = not self._odcr_filter
            label = self.query_one("#filters", Label)
            if self._odcr_filter:
                label.update(f"  [ODCR-only filter ON — press f to clear]  |  [s] save  [q] quit")
            else:
                label.update(
                    f"  Account: {self._account_id}   |   "
                    f"[f] toggle ODCR-only filter   |   [enter] show CB pricing   |   [s] save report"
                )
            self._refresh_table()
            panel = self.query_one("#detail-panel", Static)
            panel.remove_class("visible")

        def on_data_table_row_selected(self, event: DataTable.RowSelected):
            visible = self._visible_results()
            if event.cursor_row < len(visible):
                self._selected_row = visible[event.cursor_row]

        def action_expand(self):
            r = self._selected_row
            if not r:
                return
            panel = self.query_one("#detail-panel", Static)
            if not r.cb_offerings:
                msg = r.cb_error or "No Capacity Block offerings available."
                panel.update(f"[bold]{r.instance_type}[/bold] {r.az}  —  CB: {msg}")
            else:
                lines = [f"[bold]{r.instance_type}[/bold] in {r.az} — Capacity Block pricing:\n"]
                for cb in r.cb_offerings:
                    weeks = cb.duration_hours // 168
                    monthly = int(cb.upfront_fee / (cb.duration_hours / 24 / 30.44))
                    lines.append(f"  {weeks:2}w  |  Start: {cb.start_date}  End: {cb.end_date}  |  ${int(cb.upfront_fee):,} upfront  (~${monthly:,}/mo)")
                panel.update("\n".join(lines))
            panel.add_class("visible")

        def action_save(self):
            self.push_screen(SaveScreen(self._all_results, self._gpu_specs, self._account_id))

    GpuFinderApp(results, gpu_specs, account_id).run()


# ---------------------------------------------------------------------------
# Plain terminal fallback
# ---------------------------------------------------------------------------

def print_plain_report(results: list[CapacityResult], gpu_specs: dict[str, GpuSpec]):
    try:
        from rich.console import Console
        from rich.table import Table
        console = Console()
        table = Table(title="GPU Capacity Results", show_lines=True)
        table.add_column("Instance", style="cyan")
        table.add_column("GPUs")
        table.add_column("Region")
        table.add_column("AZ")
        table.add_column("ODCR")
        table.add_column("CB Start")
        table.add_column("CB 1w")
        table.add_column("CB 4w")
        for r in results:
            spec = gpu_specs.get(r.instance_type)
            gpu_str = spec.summary if spec else "—"
            cb1_offer = next((cb for cb in r.cb_offerings if cb.duration_hours == 168), None)
            cb4_offer = next((cb for cb in r.cb_offerings if cb.duration_hours == 672), None)
            cb_start = cb1_offer.start_date if cb1_offer else "—"
            cb1 = f"${int(cb1_offer.upfront_fee):,}" if cb1_offer else "—"
            cb4 = f"${int(cb4_offer.upfront_fee):,}" if cb4_offer else "—"
            status_style = "green" if "Confirmed" in r.odcr_status else ("red" if "Insufficient" in r.odcr_status else "yellow")
            status_text = r.odcr_status
            if r.odcr_detail and "Error" in r.odcr_status:
                status_text = f"{r.odcr_status}\n{r.odcr_detail}"
            table.add_row(r.instance_type, gpu_str, r.region, r.az.split("-")[-1],
                          f"[{status_style}]{status_text}[/{status_style}]", cb_start, cb1, cb4)
        console.print(table)
    except ImportError:
        for r in results:
            status = r.odcr_status
            if r.odcr_detail and "Error" in r.odcr_status:
                status = f"{r.odcr_status} ({r.odcr_detail})"
            print(f"{r.instance_type:20} {r.region:20} {r.az:25} ODCR: {status}")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(description="GPU Capacity Finder")
    parser.add_argument("--check-auth", action="store_true", help="Check auth and exit")
    parser.add_argument("--no-tui", action="store_true", help="Skip TUI, print plain table")
    parser.add_argument("--all", action="store_true", help="Scan all accessible regions, all instances, save all formats")
    args = parser.parse_args()

    print("=" * 60)
    print("  GPU Capacity Finder")
    print("=" * 60)
    identity = check_auth()
    account_id = identity["Account"]
    
    print("=" * 60)
    # Perform warning: 
    print("\n[WARNING]: This tool performs real capacity reservation attempts (which do not succeed but do test capacity), may have some billing implications, and makes multiple API calls. Use with caution and consider using --check-auth to verify credentials without scanning.")
    print("\nA Capacity Reservation cost is incurred if the reservation is successful, but since this tool immediately cancels any successful reservations, you should not be charged or a dollar charged. However, if you have very tight capacity in your account and a reservation succeeds, it could temporarily reduce your available capacity until the cancellation is processed. Additionally, there may be API rate limits to consider if scanning many regions/instances.")
    print("=" * 60)
    print("\n Do you want to proceed? (y/N)")
    proceed = input("> ").strip().lower()
    if proceed != "y":
        print("Aborting.")
        return
    
    

    if args.check_auth:
        return

    print("\nChecking region access...")
    region_status: list[tuple[str, str, bool]] = []
    with ThreadPoolExecutor(max_workers=5) as ex:
        futures = {ex.submit(check_region_access, code): (code, name) for code, name in CANDIDATE_REGIONS}
        for fut in as_completed(futures):
            code, name = futures[fut]
            ok = fut.result()
            region_status.append((code, name, ok))
    region_status.sort(key=lambda x: CANDIDATE_REGIONS.index((x[0], x[1])) if (x[0], x[1]) in CANDIDATE_REGIONS else 99)

    if args.all:
        selected_regions = [code for code, _, ok in region_status if ok]
        selected_instances = P_SERIES + G_SERIES
    else:
        selected_regions = select_regions(region_status)
        print("\nFinding available GPUs...")
        base_specs = fetch_gpu_specs(ALL_CANDIDATES, selected_regions[0] if selected_regions else "us-east-1")
        selected_instances = select_instances(base_specs)

    print("\nFetching GPU specs...")
    gpu_specs = fetch_gpu_specs(ALL_CANDIDATES, selected_regions[0] if selected_regions else "us-east-1")
    for region in selected_regions:
        gpu_specs.update(fetch_gpu_specs_for_region(selected_instances, region))

    total = len(selected_regions) * len(selected_instances)
    done = [0]
    print(f"\nScanning {total} region/instance combinations (parallel)...")

    try:
        from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn
        with Progress(SpinnerColumn(), TextColumn("{task.description}"), BarColumn(), TaskProgressColumn()) as progress:
            task = progress.add_task("Scanning...", total=total)

            def on_progress(r, i):
                done[0] += 1
                progress.update(task, advance=1, description=f"[cyan]{r}[/] {i}")

            results = run_scan(selected_regions, selected_instances, on_progress)
    except ImportError:
        results = run_scan(selected_regions, selected_instances)

    print(f"\nScan complete — {len(results)} AZ/instance combinations checked.")

    if args.all:
        out_dir = os.path.dirname(os.path.abspath(__file__))
        saved = save_outputs(results, gpu_specs, account_id, ["md", "json", "html"], out_dir)
        for p in saved:
            print(f"Saved: {p}")
        print_plain_report(results, gpu_specs)
        return

    if args.no_tui:
        print_plain_report(results, gpu_specs)
        print("\nSave formats? (1=md, 2=json, 3=html, 4=all, Enter=skip):")
        raw = input("> ").strip()
        fmt_map = {"1": ["md"], "2": ["json"], "3": ["html"], "4": ["md", "json", "html"]}
        fmts = fmt_map.get(raw)
        if fmts:
            out_dir = os.path.dirname(os.path.abspath(__file__))
            saved = save_outputs(results, gpu_specs, account_id, fmts, out_dir)
            for p in saved:
                print(f"Saved: {p}")
        return

    run_tui(results, gpu_specs, account_id)


if __name__ == "__main__":
    main()
