package export

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ACM-Dev/gpu-finder/internal/types"
)

func ExportMarkdown(results []types.CapacityResult, accountID string, gpuSpecs map[string]types.GpuSpec, path string) error {
	today := time.Now().Format("January 02, 2006")
	var lines []string
	lines = append(lines, "# GPU Capacity Research")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("**Date:** %s", today))
	lines = append(lines, fmt.Sprintf("**Account:** %s", accountID))
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")
	lines = append(lines, "## ODCR Results")
	lines = append(lines, "")
	lines = append(lines, "| Instance | GPUs | Region | AZ | ODCR Status |")
	lines = append(lines, "|---|---|---|---|---|")

	for _, r := range results {
		spec := gpuSpecs[r.Instance]
		gpuStr := spec.SummaryFull()
		if gpuStr == "" {
			gpuStr = "N/A"
		}
		status := r.Status
		if r.Detail != "" && strings.Contains(status, "Error") {
			status = fmt.Sprintf("%s — %s", status, r.Detail)
		}
		azShort := azShort(r.AZ)
		lines = append(lines, fmt.Sprintf("| `%s` | %s | %s | %s | %s |", r.Instance, gpuStr, r.Region, azShort, status))
	}

	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")
	lines = append(lines, "## Capacity Block Offerings")
	lines = append(lines, "")

	cbResults := resultsWithCb(results)
	if len(cbResults) > 0 {
		lines = append(lines, "| Instance | GPUs | Region | AZ | Duration | Start | End | Upfront | /month |")
		lines = append(lines, "|---|---|---|---|---|---|---|---|---|")
		for _, r := range cbResults {
			spec := gpuSpecs[r.Instance]
			gpuStr := spec.SummaryFull()
			if gpuStr == "" {
				gpuStr = "N/A"
			}
			for _, cb := range r.CbOfferings {
				weeks := cb.DurationHours / 168
				monthly := 0.0
				if cb.DurationHours > 0 {
					monthly = cb.UpfrontFee / (float64(cb.DurationHours) / 24 / 30.44)
				}
				lines = append(lines, fmt.Sprintf("| `%s` | %s | %s | %s | %dw | %s | %s | $%s | ~$%s/mo |",
					r.Instance, gpuStr, r.Region, azShort(cb.AZ), weeks, cb.StartDate, cb.EndDate,
					formatInt(int64(cb.UpfrontFee)), formatInt(int64(monthly))))
			}
		}
	} else {
		lines = append(lines, "> No Capacity Block offerings found.")
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func ExportJSON(results []types.CapacityResult, accountID string, gpuSpecs map[string]types.GpuSpec, path string) error {
	type GpuSpecJSON struct {
		Count     int     `json:"count"`
		Name      string  `json:"name"`
		PerGpuGB  float64 `json:"per_gpu_gb"`
		TotalGB   float64 `json:"total_gb"`
		Vcpus     int     `json:"vcpus"`
	}

	type CbJSON struct {
		DurationHours int    `json:"duration_hours"`
		DurationWeeks int    `json:"duration_weeks"`
		StartDate     string `json:"start_date"`
		EndDate       string `json:"end_date"`
		UpfrontFeeUSD float64 `json:"upfront_fee_usd"`
		AZ            string `json:"az"`
	}

	type ResultJSON struct {
		Region          string      `json:"region"`
		InstanceType    string      `json:"instance_type"`
		AZ              string      `json:"az"`
		GpuSpec         *GpuSpecJSON `json:"gpu_spec"`
		OdcrStatus      string      `json:"odcr_status"`
		OdcrDetail      string      `json:"odcr_detail"`
		CapacityBlocks  []CbJSON    `json:"capacity_blocks"`
		CbError         string      `json:"cb_error"`
	}

	type Output struct {
		Account string       `json:"account"`
		Date    string       `json:"date"`
		Results []ResultJSON `json:"results"`
	}

	out := Output{
		Account: accountID,
		Date:    time.Now().Format("2006-01-02"),
	}

	for _, r := range results {
		res := ResultJSON{
			Region:       r.Region,
			InstanceType: r.Instance,
			AZ:           r.AZ,
			OdcrStatus:   r.Status,
			OdcrDetail:   r.Detail,
			CbError:      r.CbError,
		}

		if spec, ok := gpuSpecs[r.Instance]; ok {
			res.GpuSpec = &GpuSpecJSON{
				Count:    spec.GpuCount,
				Name:     spec.GpuName,
				PerGpuGB: float64(spec.PerGpuMiB) / 1024,
				TotalGB:  float64(spec.TotalGpuMiB) / 1024,
				Vcpus:    spec.Vcpus,
			}
		}

		for _, cb := range r.CbOfferings {
			res.CapacityBlocks = append(res.CapacityBlocks, CbJSON{
				DurationHours: cb.DurationHours,
				DurationWeeks: cb.DurationHours / 168,
				StartDate:     cb.StartDate,
				EndDate:       cb.EndDate,
				UpfrontFeeUSD: cb.UpfrontFee,
				AZ:            cb.AZ,
			})
		}

		out.Results = append(out.Results, res)
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ExportHTML(results []types.CapacityResult, accountID string, gpuSpecs map[string]types.GpuSpec, prices map[string]float64, path string) error {
	today := time.Now().Format("January 02, 2006")

	var rows strings.Builder
	for _, r := range results {
		spec := gpuSpecs[r.Instance]
		gpuStr := spec.SummaryFull()
		if gpuStr == "" {
			gpuStr = "—"
		}

		cb1 := findCbOffering(r.CbOfferings, 168)
		cbStr := "—"
		if cb1 != nil {
			cbStr = fmt.Sprintf("%s / $%s/1w", cb1.StartDate, formatInt(int64(cb1.UpfrontFee)))
		} else if r.CbError != "" {
			cbStr = r.CbError
		}

		color := statusColor(r.Status)
		statusText := r.Status
		if r.Detail != "" && strings.Contains(r.Status, "Error") {
			statusText = fmt.Sprintf("%s<br><small>%s</small>", r.Status, r.Detail)
		}

		priceStr := "—"
		if price, ok := prices[fmt.Sprintf("%s/%s", r.Region, r.Instance)]; ok {
			priceStr = fmt.Sprintf("$%.4f/hr", price)
		}

		rows.WriteString(fmt.Sprintf("<tr>"+
			"<td><code>%s</code></td>"+
			"<td>%s</td>"+
			"<td>%s</td>"+
			"<td>%s</td>"+
			"<td style='color:%s;font-weight:600'>%s</td>"+
			"<td>%s</td>"+
			"<td>%s</td>"+
			"</tr>\n",
			r.Instance, gpuStr, r.Region, azShort(r.AZ), color, statusText, cbStr, priceStr))
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GPU Capacity Report — %s</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background: #f8fafc; color: #1a1a1a; margin: 40px; line-height: 1.5; }
  h1 { color: #1F4E79; border-bottom: 3px solid #1F4E79; padding-bottom: 8px; margin-bottom: 16px; }
  h2 { color: #1F4E79; margin-top: 32px; margin-bottom: 12px; }
  table { border-collapse: collapse; width: 100%%; margin-top: 12px; background: #fff; box-shadow: 0 1px 3px rgba(0,0,0,0.1); border-radius: 8px; overflow: hidden; }
  th { background: #1F4E79; color: #fff; padding: 10px 14px; text-align: left; font-size: 13px; font-weight: 600; }
  td { padding: 8px 14px; font-size: 13px; border-bottom: 1px solid #e5e7eb; }
  tr:nth-child(even) td { background: #F0F4FA; }
  tr:hover td { background: #E8EEF6; }
  code { background: #EEF3F8; padding: 2px 6px; border-radius: 3px; color: #1F4E79; font-size: 12px; }
  .meta { color: #4472C4; font-size: 14px; margin-bottom: 24px; display: flex; gap: 24px; }
  .meta span { background: #fff; padding: 4px 12px; border-radius: 4px; box-shadow: 0 1px 2px rgba(0,0,0,0.05); }
  .container { max-width: 1400px; margin: 0 auto; }
</style>
</head>
<body>
<div class="container">
<h1>GPU Capacity Research</h1>
<div class="meta">
  <span><strong>Date:</strong> %s</span>
  <span><strong>Account:</strong> %s</span>
</div>
<h2>ODCR &amp; Capacity Block Results</h2>
<table>
<tr>
  <th>Instance</th><th>GPUs</th><th>Region</th><th>AZ</th><th>ODCR Status</th><th>CB Earliest Start / 1w Price</th><th>On-Demand</th>
</tr>
%s
</table>
</div>
</body>
</html>`, today, today, accountID, rows.String())

	return os.WriteFile(path, []byte(html), 0644)
}

func PrintPlainTable(results []types.CapacityResult, gpuSpecs map[string]types.GpuSpec, prices map[string]float64) {
	fmt.Println("\n=== GPU Capacity Results ===")
	fmt.Printf("%-18s %-30s %-14s %-14s %-22s %-12s %-12s\n",
		"Instance", "GPUs", "Region", "AZ", "ODCR Status", "CB 1w", "Price/hr")

	for _, r := range results {
		spec := gpuSpecs[r.Instance]
		gpuStr := spec.SummaryFull()
		if gpuStr == "" {
			gpuStr = "—"
		}

		cb1 := findCbOffering(r.CbOfferings, 168)
		cb1w := "—"
		if cb1 != nil {
			cb1w = fmt.Sprintf("$%s", formatInt(int64(cb1.UpfrontFee)))
		}

		priceStr := "—"
		if price, ok := prices[fmt.Sprintf("%s/%s", r.Region, r.Instance)]; ok {
			priceStr = fmt.Sprintf("$%.4f", price)
		}

		fmt.Printf("%-18s %-30s %-14s %-14s %-22s %-12s %-12s\n",
			r.Instance, gpuStr, r.Region, azShort(r.AZ), r.Status, cb1w, priceStr)
	}
}

func resultsWithCb(results []types.CapacityResult) []types.CapacityResult {
	var out []types.CapacityResult
	for _, r := range results {
		if len(r.CbOfferings) > 0 {
			out = append(out, r)
		}
	}
	return out
}

func findCbOffering(offerings []types.CbOffering, hours int) *types.CbOffering {
	for i := range offerings {
		if offerings[i].DurationHours == hours {
			return &offerings[i]
		}
	}
	return nil
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(c)
	}
	if negative {
		return "-" + result.String()
	}
	return result.String()
}

func azShort(az string) string {
	parts := strings.Split(az, "-")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return az
}

func statusColor(status string) string {
	if strings.Contains(status, "Confirmed") {
		return "#1a7a3c"
	}
	if strings.Contains(status, "Insufficient") {
		return "#b91c1c"
	}
	if strings.Contains(status, "Unsupported") {
		return "#6b7280"
	}
	if strings.Contains(status, "Quota") || strings.Contains(status, "Limit") {
		return "#b45309"
	}
	return "#374151"
}
