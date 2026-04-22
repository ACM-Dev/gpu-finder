package tui

import (
	"fmt"
	"strings"

	"github.com/ACM-Dev/gpu-finder/internal/styles"
	"github.com/ACM-Dev/gpu-finder/internal/types"

	"github.com/charmbracelet/lipgloss"
)

func View(m types.Model) string {
	if m.ErrorMsg != "" {
		return styles.BaseStyle.Render(styles.ErrorStyle.Render("Error: "+m.ErrorMsg))
	}

	s := strings.Builder{}
	s.WriteString(styles.TitleStyle.Render(" AWS GPU Capacity Finder ") + "\n")
	if m.AccountID != "" {
		s.WriteString(styles.DimStyle.Render(fmt.Sprintf("Account : %s", m.AccountID)) + "\n")
		s.WriteString(styles.DimStyle.Render(fmt.Sprintf("ARN     : %s", m.Arn)) + "\n")
		if m.OrgID != "" {
			s.WriteString(styles.DimStyle.Render(fmt.Sprintf("Org     : %s (master: %s / %s)", m.OrgID, m.OrgMasterID, m.OrgMasterEmail)) + "\n")
		} else {
			s.WriteString(styles.DimStyle.Render("Org     : (no organizations access)") + "\n")
		}
		s.WriteString("\n")
	}

	switch m.State {
	case types.StateWelcome:
		s.WriteString(styles.HighlightStyle.Render("Welcome to") + "\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900")).Bold(true).Render(`
██████╗ ██████╗ ██╗   ██╗    ███████╗██╗███╗   ██╗██████╗ ███████╗██████╗ 
██╔════╝ ██╔══██╗██║   ██║    ██╔════╝██║████╗  ██║██╔══██╗██╔════╝██╔══██╗
██║  ███╗██████╔╝██║   ██║    █████╗  ██║██╔██╗ ██║██║  ██║█████╗  ██████╔╝
██║   ██║██╔═══╝ ██║   ██║    ██╔══╝  ██║██║╚██╗██║██║  ██║██╔══╝  ██╔══██╗
╚██████╔╝██║     ╚██████╔╝    ██║     ██║██║ ╚████║██████╔╝███████╗██║  ██║
 ╚═════╝ ╚═╝      ╚═════╝     ╚═╝     ╚═╝╚═╝  ╚═══╝╚═════╝ ╚══════╝╚═╝  ╚═╝ 
`) + "\n")
		s.WriteString(styles.SubTitleStyle.Render("TERMS OF USE & POLICY:") + "\n")
		s.WriteString(styles.DimStyle.Render("1. NO RESPONSIBILITY: We are not liable for any billing charges.") + "\n")
		s.WriteString(styles.DimStyle.Render("2. AS-IS: Results are best-effort and do not guarantee future capacity.") + "\n")
		s.WriteString(styles.DimStyle.Render("3. COMPLIANCE: Ensure you have permission to perform these actions.") + "\n\n")
		s.WriteString(styles.WarnStyle.Render("WARNING: This tool performs REAL capacity reservation attempts.") + "\n")
		s.WriteString("Each successful reservation is immediately cancelled, but\n")
		s.WriteString("it may briefly incur billing costs or impact your service quotas.\n\n")
		s.WriteString(styles.HighlightStyle.Render("Press ENTER to accept and continue...") + "\n")

	case types.StateInitializing:
		s.WriteString(m.Spinner.View() + " ")
		s.WriteString(styles.HighlightStyle.Render("Initializing AWS Session & Loading Regions..."))

	case types.StateRegionSelect:
		s.WriteString(styles.SubTitleStyle.Render("Select Target Regions (Space to toggle, Enter to confirm):") + "\n")
		s.WriteString(styles.DimStyle.Render("Note: * marks your default region") + "\n\n")
		s.WriteString(styles.DimStyle.Render(fmt.Sprintf("    %-4s %-20s %-18s %s", "SEL", "REGION", "NAME", "STATUS")) + "\n")

		for i, r := range m.Regions {
			cursorCol := " "
			if m.Cursor == i {
				cursorCol = ">"
			}
			checkCol := "[ ]"
			if r.Selected {
				checkCol = "[x]"
			}

			idCol := r.ID
			if r.IsDefault {
				idCol += " *"
			}

			statusEmoji := "✅ "
			cursorStyle := lipgloss.NewStyle()
			checkStyle := lipgloss.NewStyle()
			idStyle := lipgloss.NewStyle().Width(20)
			nameStyle := lipgloss.NewStyle().Width(18)
			statusStyle := lipgloss.NewStyle()

			if r.Disabled {
				statusEmoji = "❌ "
				cursorStyle = styles.DimStyle
				checkStyle = styles.DimStyle
				idStyle = idStyle.Strikethrough(true).Foreground(lipgloss.Color("#777777"))
				nameStyle = nameStyle.Strikethrough(true).Foreground(lipgloss.Color("#777777"))
				statusStyle = statusStyle.Foreground(lipgloss.Color("#FF4444"))
			} else if m.Cursor == i {
				cursorStyle = styles.HighlightStyle
				checkStyle = styles.HighlightStyle
				idStyle = styles.HighlightStyle.Width(20)
				nameStyle = styles.HighlightStyle.Width(18)
				statusStyle = styles.HighlightStyle
			}

			line := fmt.Sprintf("%s %s  %s %s %s",
				cursorStyle.Render(cursorCol),
				checkStyle.Render(checkCol),
				idStyle.Render(idCol),
				nameStyle.Render(r.Name),
				statusStyle.Render(statusEmoji+r.Detail),
			)
			s.WriteString(line + "\n")
		}

	case types.StateLoadingInstances:
		s.WriteString(m.Spinner.View() + " ")
		s.WriteString(styles.HighlightStyle.Render("Fetching available GPU & Accelerator instance types..."))

	case types.StateInstanceSelect:
		var selectedCodes []string
		for _, r := range m.Regions {
			if r.Selected && !r.Disabled {
				selectedCodes = append(selectedCodes, r.ID)
			}
		}
		s.WriteString(styles.SubTitleStyle.Render("Select Instance Types (Space to toggle, Enter to start scan):") + "\n")
		s.WriteString(styles.DimStyle.Render("Targeting Regions: "+strings.Join(selectedCodes, ", ")) + "\n\n")
		s.WriteString(styles.DimStyle.Render(fmt.Sprintf("    %-4s %-20s %s", "SEL", "INSTANCE TYPE", "GPU SPECIFICATION")) + "\n")

		for i, it := range m.Instances {
			cursorCol := " "
			if m.Cursor == i {
				cursorCol = ">"
			}
			checkCol := "[ ]"
			if it.Selected {
				checkCol = "[x]"
			}

			cursorStyle := lipgloss.NewStyle()
			checkStyle := lipgloss.NewStyle()
			idStyle := lipgloss.NewStyle().Width(20)
			detailStyle := lipgloss.NewStyle()

			if m.Cursor == i {
				cursorStyle = styles.HighlightStyle
				checkStyle = styles.HighlightStyle
				idStyle = styles.HighlightStyle.Width(20)
				detailStyle = styles.HighlightStyle
			}

			line := fmt.Sprintf("%s %s  %s %s",
				cursorStyle.Render(cursorCol),
				checkStyle.Render(checkCol),
				idStyle.Render(it.ID),
				detailStyle.Render(it.Detail),
			)
			s.WriteString(line + "\n")
		}

	case types.StateScanning:
		s.WriteString(styles.WarnStyle.Render("Scanning AWS Capacity... (ODCR + Capacity Block checks)") + "\n\n")
		s.WriteString(m.Progress.View() + "\n\n")
		s.WriteString(styles.DimStyle.Render(m.ScanningMsg) + "\n")

	case types.StateDone:
		s.WriteString(styles.SuccessStyle.Render(fmt.Sprintf("Scan Complete! Found %d combinations.", len(m.Results))) + "\n\n")
		s.WriteString(m.Table.View() + "\n")

		if m.DetailVisible && m.SelectedRow != nil {
			s.WriteString(renderDetailPanel(m) + "\n")
		}

		if m.SaveStatus != "" {
			s.WriteString("\n" + styles.HighlightStyle.Render(m.SaveStatus))
		}

	case types.StateAuthDone:
		s.WriteString(styles.SuccessStyle.Render("AWS Authentication Successful") + "\n\n")
		s.WriteString(styles.SubTitleStyle.Render("Account Details:") + "\n\n")
		s.WriteString(fmt.Sprintf("  Account : %s\n", styles.HighlightStyle.Render(m.AccountID)))
		s.WriteString(fmt.Sprintf("  ARN     : %s\n", styles.DimStyle.Render(m.Arn)))
		if m.OrgID != "" {
			s.WriteString(fmt.Sprintf("  Org     : %s\n", styles.DimStyle.Render(fmt.Sprintf("%s (master: %s / %s)", m.OrgID, m.OrgMasterID, m.OrgMasterEmail))))
		} else {
			s.WriteString(fmt.Sprintf("  Org     : %s\n", styles.DimStyle.Render("(no organizations access)")))
		}
		s.WriteString("\n" + styles.DimStyle.Render("Press ENTER or q to exit") + "\n")
	}

	var footer string
	switch m.State {
	case types.StateWelcome:
		footer = "enter: accept • q: quit"
	case types.StateRegionSelect:
		footer = "↑/↓: nav • a: all • n: none • space: toggle • enter: confirm • q: quit"
	case types.StateInstanceSelect:
		footer = "↑/↓: nav • a: all • n: none • space: toggle • esc: back • enter: scan • q: quit"
	case types.StateScanning:
		footer = "scanning... please wait • q: quit"
	case types.StateDone:
		if m.ShowSavePrompt {
			footer = "1: md • 2: json • 3: html • 4: all • esc: cancel"
		} else {
			footer = "f: filter • d: detail • enter: select row • s: save • q: quit"
		}
	case types.StateAuthDone:
		footer = "enter: exit • q: quit"
	default:
		footer = "q: quit"
	}

	content := styles.BaseStyle.Render(s.String())

	if m.ShowSavePrompt {
		saveBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF9900")).
			Padding(1, 3).
			Render(styles.SubTitleStyle.Render("Save Report — Choose Format:") + "\n\n" +
				styles.HighlightStyle.Render("1") + ". Markdown (.md)\n" +
				styles.HighlightStyle.Render("2") + ". JSON (.json)\n" +
				styles.HighlightStyle.Render("3") + ". HTML (.html)\n" +
				styles.HighlightStyle.Render("4") + ". All formats\n\n" +
				styles.DimStyle.Render("esc: cancel"))
		content = lipgloss.NewStyle().
			Width(80).
			Render(content + "\n\n" + lipgloss.NewStyle().PaddingLeft(2).Render(saveBox))
	}

	if m.ShowQuitConfirm {
		quitBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF9900")).
			Padding(1, 3).
			Render(styles.WarnStyle.Render("Really quit?") + "\n\n" +
				styles.HighlightStyle.Render("y") + "es / " + styles.HighlightStyle.Render("n") + "o")
		content = lipgloss.NewStyle().
			Width(80).
			Render(content + "\n\n" + lipgloss.NewStyle().PaddingLeft(2).Render(quitBox))
	}

	return content + "\n" + styles.FooterStyle.Render(" "+footer+" • made by acuitmeshdev ")
}

func renderDetailPanel(m types.Model) string {
	r := m.SelectedRow
	if r == nil {
		return ""
	}

	lines := []string{
		fmt.Sprintf("[bold]%s[/] in %s (%s)", r.Instance, r.AZ, r.Region),
		fmt.Sprintf("ODCR Status: %s", r.Status),
		"",
	}

	if r.Detail != "" {
		lines = append(lines, fmt.Sprintf("Detail: %s", r.Detail))
		lines = append(lines, "")
	}

	if len(r.CbOfferings) == 0 {
		if r.CbError != "" {
			lines = append(lines, fmt.Sprintf("Capacity Blocks: %s", r.CbError))
		} else {
			lines = append(lines, "Capacity Blocks: No offerings available.")
		}
	} else {
		lines = append(lines, "Capacity Block Pricing:")
		for _, cb := range r.CbOfferings {
			weeks := cb.DurationHours / 168
			monthly := 0.0
			if cb.DurationHours > 0 {
				monthly = cb.UpfrontFee / (float64(cb.DurationHours) / 24 / 30.44)
			}
			lines = append(lines, fmt.Sprintf("  %2dw | Start: %s | End: %s | $%s upfront (~$%s/mo)",
				weeks, cb.StartDate, cb.EndDate, formatFee(cb.UpfrontFee), formatFee(monthly)))
		}
	}

	if spec, ok := m.GpuSpecs[r.Instance]; ok {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("GPU Specs: %s", spec.SummaryFull()))
	}

	if price, ok := m.Prices[fmt.Sprintf("%s/%s", r.Region, r.Instance)]; ok {
		lines = append(lines, fmt.Sprintf("On-Demand Price: $%.4f/hr", price))
	}

	content := strings.Join(lines, "\n")
	return styles.DetailStyle.Render(content)
}
