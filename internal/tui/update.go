package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ACM-Dev/gpu-finder/internal/api"
	"github.com/ACM-Dev/gpu-finder/internal/export"
	"github.com/ACM-Dev/gpu-finder/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func InitialModel(mode string) types.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	return types.Model{
		State:    types.StateWelcome,
		Progress: newProgress(),
		Spinner:  s,
		Mode:     mode,
	}
}

func Update(m types.Model, msg tea.Msg) (types.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.ShowQuitConfirm {
			if msg.String() == "y" || msg.String() == "Y" {
				return m, tea.Quit
			}
			if msg.String() == "n" || msg.String() == "N" || msg.String() == "esc" {
				m.ShowQuitConfirm = false
				return m, nil
			}
			return m, nil
		}

		if m.ShowSavePrompt {
			fmtMap := map[string][]string{"1": {"md"}, "2": {"json"}, "3": {"html"}, "4": {"md", "json", "html"}}
			if fmts, ok := fmtMap[msg.String()]; ok {
				m.ShowSavePrompt = false
				m.SaveStatus = "Saving..."
				return m, saveReportCmd(m.Results, m.AccountID, m.GpuSpecs, m.Prices, fmts)
			}
			if msg.String() == "esc" || msg.String() == "enter" {
				m.ShowSavePrompt = false
				return m, nil
			}
			return m, nil
		}

		if msg.String() == "q" {
			m.ShowQuitConfirm = true
			return m, nil
		}

		if m.State == types.StateAuthDone {
			if msg.String() == "enter" || msg.String() == "q" {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.State == types.StateWelcome && msg.String() == "enter" {
			m.State = types.StateInitializing
			m.Spinner = NewSpinner()
			return m, tea.Batch(checkAuthCmd(), m.Spinner.Tick)
		}

		if (msg.String() == "esc" || msg.String() == "backspace") && (m.State == types.StateInstanceSelect || m.State == types.StateLoadingInstances) {
			m.State = types.StateRegionSelect
			m.Cursor = 0
			return m, nil
		}

		if m.State == types.StateDone {
			if msg.String() == "f" {
				m.FilterConfirmed = !m.FilterConfirmed
				BuildTable(&m)
				return m, nil
			}
			if msg.String() == "d" {
				rows := getFilteredResults(m.Results, m.FilterConfirmed)
				if len(rows) > 0 {
					if m.SelectedRow == nil {
						m.SelectedRow = &rows[0]
					}
					m.DetailVisible = !m.DetailVisible
				}
				return m, nil
			}
			if msg.String() == "s" {
				m.ShowSavePrompt = true
				return m, nil
			}
			if msg.String() == "enter" && len(getFilteredResults(m.Results, m.FilterConfirmed)) > 0 {
				rowIdx := m.Table.Cursor()
				rows := getFilteredResults(m.Results, m.FilterConfirmed)
				if rowIdx < len(rows) {
					m.SelectedRow = &rows[rowIdx]
				}
				return m, nil
			}
		}

		if m.State == types.StateRegionSelect || m.State == types.StateInstanceSelect {
			var list *[]types.CheckableItem
			if m.State == types.StateRegionSelect {
				list = &m.Regions
			} else {
				list = &m.Instances
			}
			switch msg.String() {
			case "up", "k":
				if m.Cursor > 0 {
					m.Cursor--
				}
			case "down", "j":
				if m.Cursor < len(*list)-1 {
					m.Cursor++
				}
			case "a":
				for i := range *list {
					if !(*list)[i].Disabled {
						(*list)[i].Selected = true
					}
				}
			case "n":
				for i := range *list {
					(*list)[i].Selected = false
				}
			case " ", "x":
				if !(*list)[m.Cursor].Disabled {
					(*list)[m.Cursor].Selected = !(*list)[m.Cursor].Selected
				}
			case "enter":
				m.Cursor = 0
				if m.State == types.StateRegionSelect {
					var sel []string
					for _, r := range m.Regions {
						if r.Selected && !r.Disabled {
							sel = append(sel, r.ID)
						}
					}
					if len(sel) == 0 {
						return m, nil
					}
					m.State = types.StateLoadingInstances
					m.Spinner = NewSpinner()
					return m, tea.Batch(LoadInstancesCmd(m.AwsCfg, sel), m.Spinner.Tick)
				} else {
					rCount, iCount := 0, 0
					for _, r := range m.Regions {
						if r.Selected && !r.Disabled {
							rCount++
						}
					}
					for _, i := range m.Instances {
						if i.Selected {
							iCount++
						}
					}
					if iCount == 0 {
						return m, nil
					}
					m.TotalJobs = rCount * iCount
					m.State = types.StateScanning
					return m, nil
				}
			}
		}

	case types.AuthMsg:
		if msg.Err != nil {
			m.ErrorMsg = msg.Err.Error()
			return m, nil
		}
		m.AwsCfg = msg.Cfg
		m.AccountID = msg.AccountID
		m.Arn = msg.Arn
		m.OrgID = msg.OrgID
		m.OrgMasterID = msg.OrgMasterID
		m.OrgMasterEmail = msg.OrgMasterEmail
		if m.Mode == "auth" {
			m.State = types.StateAuthDone
			return m, nil
		}
		m.Spinner = NewSpinner()
		return m, tea.Batch(LoadRegionsCmd(m.AwsCfg), m.Spinner.Tick)

	case types.RegionsLoadedMsg:
		m.Regions = msg
		m.State = types.StateRegionSelect
		m.Cursor = 0

	case types.InstancesLoadedMsg:
		m.Instances = msg
		m.State = types.StateInstanceSelect
		m.Cursor = 0

	case types.ScanProgressMsg:
		if msg.Result != nil {
			m.Results = append(m.Results, *msg.Result)
		} else if msg.JobName == "" {
			m.Completed++
		} else {
			m.ScanningMsg = msg.JobName
		}
		if m.TotalJobs > 0 {
			return m, m.Progress.SetPercent(float64(m.Completed) / float64(m.TotalJobs))
		}
		return m, nil

	case progress.FrameMsg:
		newProg, cmd := m.Progress.Update(msg)
		m.Progress = newProg.(progress.Model)
		return m, cmd

	case spinner.TickMsg:
		newSpin, cmd := m.Spinner.Update(msg)
		m.Spinner = newSpin
		return m, cmd

	case types.ScanDoneMsg:
		m.State = types.StateDone
		fetchPostScanData(&m)
		BuildTable(&m)

	case types.SaveDoneMsg:
		m.SaveStatus = string(msg)
	}

	if m.State == types.StateInitializing || m.State == types.StateLoadingInstances {
		return m, tea.Batch(m.Spinner.Tick)
	}
	return m, nil
}

func fetchPostScanData(m *types.Model) {
	confirmedSet := make(map[string]bool)
	var regions []string
	regionSet := make(map[string]bool)
	for _, r := range m.Results {
		if strings.Contains(r.Status, "Confirmed") {
			key := r.Instance
			if !confirmedSet[key] {
				confirmedSet[key] = true
			}
			if !regionSet[r.Region] {
				regionSet[r.Region] = true
				regions = append(regions, r.Region)
			}
		}
	}

	var confirmedInstances []string
	for k := range confirmedSet {
		confirmedInstances = append(confirmedInstances, k)
	}

	if len(confirmedInstances) > 0 && len(regions) > 0 {
		m.GpuSpecs = api.FetchGpuSpecs(confirmedInstances, regions[0])
		m.Prices = api.FetchOndemandPrices(confirmedInstances, regions)
	}
}

func BuildTable(m *types.Model) {
	columns := []table.Column{
		{Title: "Instance", Width: 16},
		{Title: "GPUs", Width: 28},
		{Title: "Region", Width: 14},
		{Title: "AZ", Width: 14},
		{Title: "ODCR Status", Width: 22},
		{Title: "CB Start", Width: 12},
		{Title: "CB 1w", Width: 12},
		{Title: "CB 4w", Width: 12},
	}

	rows := getFilteredResults(m.Results, m.FilterConfirmed)

	sort.Slice(rows, func(i, j int) bool {
		iConfirmed := strings.Contains(rows[i].Status, "Confirmed")
		jConfirmed := strings.Contains(rows[j].Status, "Confirmed")
		if iConfirmed != jConfirmed {
			return iConfirmed
		}
		if rows[i].Region != rows[j].Region {
			return rows[i].Region < rows[j].Region
		}
		return rows[i].Instance < rows[j].Instance
	})

	var tableRows []table.Row
	for _, r := range rows {
		spec := m.GpuSpecs[r.Instance]
		gpuStr := spec.SummaryFull()
		if gpuStr == "" {
			gpuStr = "—"
		}

		cb1 := findCbOffering(r.CbOfferings, 168)
		cb4 := findCbOffering(r.CbOfferings, 672)
		cbStart := "—"
		if cb1 != nil {
			cbStart = cb1.StartDate
		}
		cb1w := "—"
		if cb1 != nil {
			cb1w = fmt.Sprintf("$%s", formatFee(cb1.UpfrontFee))
		}
		cb4w := "—"
		if cb4 != nil {
			cb4w = fmt.Sprintf("$%s", formatFee(cb4.UpfrontFee))
		}

		statusStr := statusText(r.Status)

		tableRows = append(tableRows, table.Row{
			r.Instance, gpuStr, r.Region, r.AZ, statusStr, cbStart, cb1w, cb4w,
		})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(tableRows),
		table.WithFocused(true),
		table.WithHeight(minInt(15, len(tableRows)+1)),
	)
	t.SetStyles(tableStyles())
	m.Table = t
}

func getFilteredResults(results []types.CapacityResult, filterConfirmed bool) []types.CapacityResult {
	if !filterConfirmed {
		return results
	}
	var filtered []types.CapacityResult
	for _, r := range results {
		if strings.Contains(r.Status, "Confirmed") {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func findCbOffering(offerings []types.CbOffering, hours int) *types.CbOffering {
	for i := range offerings {
		if offerings[i].DurationHours == hours {
			return &offerings[i]
		}
	}
	return nil
}

func formatFee(fee float64) string {
	if fee == 0 {
		return "0"
	}
	intPart := int64(fee)
	return fmt.Sprintf("%d", intPart)
}

func statusText(status string) string {
	if strings.Contains(status, "Confirmed") {
		return "🟢 " + status
	}
	if strings.Contains(status, "Insufficient") {
		return "🔴 " + status
	}
	if strings.Contains(status, "Unsupported") {
		return "⚪ " + status
	}
	if strings.Contains(status, "Quota") || strings.Contains(status, "Limit") {
		return "🟠 " + status
	}
	return "🟡 " + status
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func newProgress() progress.Model {
	return progress.New(progress.WithGradient("#B294BB", "#81A2BE"))
}

func NewSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	return s
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	return s
}

func checkAuthCmd() tea.Cmd {
	return func() tea.Msg {
		return api.CheckAuth()
	}
}

func LoadRegionsCmd(cfg aws.Config) tea.Cmd {
	return func() tea.Msg {
		return api.LoadRegionsCmd(cfg)
	}
}

func LoadInstancesCmd(cfg aws.Config, selectedRegions []string) tea.Cmd {
	return func() tea.Msg {
		return api.LoadInstances(cfg, selectedRegions)
	}
}

func saveReportCmd(results []types.CapacityResult, accountID string, gpuSpecs map[string]types.GpuSpec, prices map[string]float64, formats []string) tea.Cmd {
	return func() tea.Msg {
		ts := time.Now().Format("2006-01-02-150405")
		base := fmt.Sprintf("gpu-report-%s", ts)

		var saved []string
		for _, f := range formats {
			switch f {
			case "md":
				path := base + ".md"
				if err := export.ExportMarkdown(results, accountID, gpuSpecs, path); err == nil {
					saved = append(saved, path)
				}
			case "json":
				path := base + ".json"
				if err := export.ExportJSON(results, accountID, gpuSpecs, path); err == nil {
					saved = append(saved, path)
				}
			case "html":
				path := base + ".html"
				if err := export.ExportHTML(results, accountID, gpuSpecs, prices, path); err == nil {
					saved = append(saved, path)
				}
			}
		}

		return types.SaveDoneMsg(fmt.Sprintf("Saved: %s", strings.Join(saved, ", ")))
	}
}
